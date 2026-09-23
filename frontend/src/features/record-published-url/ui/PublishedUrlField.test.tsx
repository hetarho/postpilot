import { Code, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { PostDraft } from '@/entities/post'
import { PostService, type AppFailureReason } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { withProviders } from '@/test/session'
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
