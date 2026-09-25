import { Code, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { usePost, type PostDraft } from '@/entities/post'
import { PostService, type AppFailureReason } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import {
  FAKE_PUBLISHED_URL,
  createFakePostsTransport,
  finalizedPostRow,
  publishedPostRow,
  type FakePostRow,
} from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { PublishedUrlField } from './PublishedUrlField'

const FINALIZED = {
  slug: 'post',
  status: 'finalized',
  publishedUrl: '',
  publishedAt: '',
} as unknown as PostDraft

function refusing(reason: AppFailureReason, code: Code) {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostPublishedUrl, () => {
      throw connectAppError(reason, code)
    })
  })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  render(<PublishedUrlField post={FINALIZED} />, { wrapper: withProviders(transport, queryClient) })
}

// A server refusal renders in place under the field in its own words, and nothing else moves:
// the field keeps what was typed and the post is not rewritten.
describe('a server refusal', () => {
  it.each([
    [
      'POST_PUBLISHED_URL_INVALID',
      Code.InvalidArgument,
      '네이버 블로그 글 주소만 저장할 수 있어요.',
    ],
    ['POST_BUSY', Code.FailedPrecondition, '이 글에서 다른 작업이 진행 중이에요.'],
    ['POST_NOT_FINALIZED', Code.FailedPrecondition, '먼저 글을 확정해 주세요.'],
  ] as const)('%s renders under the field', async (reason, code, words) => {
    const user = userEvent.setup()
    refusing(reason, code)
    const input = screen.getByLabelText('네이버 블로그 글 주소')
    await user.type(input, 'https://blog.naver.com/alice/1{Enter}')

    const message = await screen.findByRole('alert')
    expect(message).toHaveTextContent(words)
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveValue('https://blog.naver.com/alice/1')

    // A change takes the message down: the next attempt is a new question.
    await user.type(input, '2')
    expect(screen.queryByRole('alert')).toBeNull()
  })
})

// The field over the post as the cache holds it, so a landed save's answer reseeds it.
describe('the field over a stored post', () => {
  const slug = '20260820-seongsu'

  function Field() {
    const { post } = usePost(slug)
    return post ? <PublishedUrlField post={post} /> : null
  }

  function renderField(row: FakePostRow) {
    const calls: string[] = []
    const saves: string[] = []
    const transport = createFakePostsTransport({ calls, posts: [row], publishedUrlSaves: saves })
    render(<Field />, { wrapper: withProviders(transport, createTestQueryClient()) })
    return { calls, saves }
  }
  const input = () => screen.findByLabelText('네이버 블로그 글 주소')
  const save = () => screen.getByRole('button', { name: '저장' })

  it('is a url field typed for the keyboard that pastes an address', async () => {
    renderField(finalizedPostRow({ slug }))
    const field = await input()
    for (const [name, value] of Object.entries({
      type: 'url',
      inputmode: 'url',
      autocomplete: 'url',
      autocapitalize: 'none',
      autocorrect: 'off',
      enterkeyhint: 'done',
    })) {
      expect(field).toHaveAttribute(name, value)
    }
    expect(field).toHaveValue('')
    expect(save()).toBeEnabled()
    expect(screen.queryByRole('button', { name: '지우기' })).toBeNull()
  })

  it('accepts the mobile share address and shows it normalized', async () => {
    const user = userEvent.setup()
    const { saves } = renderField(finalizedPostRow({ slug }))
    await user.type(await input(), 'https://m.blog.naver.com/alice/7{Enter}')

    await waitFor(async () => expect(await input()).toHaveValue('https://blog.naver.com/alice/7'))
    // What is sent is the input as typed, trimmed; the normalization is the server's answer.
    expect(saves).toEqual(['https://m.blog.naver.com/alice/7'])
  })

  it('refuses another address in place and sends nothing', async () => {
    const user = userEvent.setup()
    const { calls } = renderField(finalizedPostRow({ slug }))
    const field = await input()
    for (const bad of [
      'https://cafe.naver.com/alice/1',
      'https://blog.naver.com:443/alice/1',
      'https://user@blog.naver.com/alice/1',
    ]) {
      await user.clear(field)
      await user.type(field, bad)
      await user.click(save())
      const message = await screen.findByRole('alert')
      expect(message).toHaveTextContent('네이버 블로그 글 주소만 저장할 수 있어요.')
      expect(field).toHaveAttribute('aria-invalid', 'true')
      expect(field.getAttribute('aria-describedby')).toContain(message.id)
    }
    expect(calls).not.toContain('SavePostPublishedUrl')
  })

  it('sends nothing for an empty 저장 on a post with no address', async () => {
    const user = userEvent.setup()
    const { calls } = renderField(finalizedPostRow({ slug }))
    await input()
    await user.click(save())
    expect(calls).not.toContain('SavePostPublishedUrl')
  })

  it('prefills a published address and sends its replacement', async () => {
    const user = userEvent.setup()
    const { saves } = renderField(publishedPostRow({ slug }))
    const field = await input()
    expect(field).toHaveValue(FAKE_PUBLISHED_URL)
    await user.clear(field)
    await user.type(field, 'https://blog.naver.com/alice/2')
    await user.click(save())

    await waitFor(async () => expect(await input()).toHaveValue('https://blog.naver.com/alice/2'))
    expect(saves).toEqual(['https://blog.naver.com/alice/2'])
  })

  it('stays closed with its reason until the post is 확정', async () => {
    const { calls } = renderField(
      finalizedPostRow({ slug, status: 'review', finalizedRevision: 0n }),
    )
    const field = await input()
    expect(field).toBeDisabled()
    expect(save()).toBeDisabled()
    const reason = screen.getByText('글을 확정하면 발행 URL을 입력할 수 있어요.')
    expect(field.getAttribute('aria-describedby')).toContain(reason.id)
    expect(calls).not.toContain('SavePostPublishedUrl')
  })
})
