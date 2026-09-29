// ③ 글 완성: what the panel holds, export, and the 발행 URL field publishing and clearing the post.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { USER, openStep, resetEditorTest } from '@/test/editor'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import {
  finalizedPostRow,
  publishedPostRow,
  type FakePostRow,
  type FakePostsOptions,
} from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('opening a post', () => {
  it('switches export formats without making another client request', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            createdAt: '2026-08-20T12:00:00Z',
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
    })

    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    // The voice profile is NOT read: the empty-profile warning belongs to 글 생성, so a post already
    // past generating no longer pays for that read.
    await waitFor(() => expect(calls).toContain('GetPost'))
    expect(calls).not.toContain('GetVoiceProfile')
    calls.length = 0

    await user.click(screen.getByRole('tab', { name: '티스토리' }))
    await user.click(screen.getByRole('tab', { name: '자체 사이트' }))
    await user.click(screen.getByRole('tab', { name: '마크다운' }))

    expect(calls).toEqual([])
  })

  it.each([
    {
      locale: 'ko' as const,
      account: { id: 'alice', plan: ProtoPlan.FREE },
      finish: '글 완성',
      exportHeading: '내보내기',
      copyTitle: '제목 복사',
    },
    {
      locale: 'en' as const,
      account: { id: 'root', plan: ProtoPlan.MASTER },
      finish: 'Finish',
      exportHeading: 'Export',
      copyTitle: 'Copy title',
    },
  ])(
    'keeps manual export and makes no publishing call for $locale/$account.plan',
    async ({ locale, account, finish, exportHeading, copyTitle }) => {
      initializeI18n(locale)
      const calls: string[] = []
      const user = userEvent.setup()
      const writeText = vi.fn(async () => undefined)
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText },
      })
      renderAppAt('/posts/20260820-jeju', {
        user: account,
        calls,
        posts: {
          posts: [
            {
              slug: '20260820-jeju',
              status: 'review',
              createdAt: '2026-08-20T12:00:00Z',
              content: POST_CONTENT_FIXTURE,
            },
          ],
        },
      })

      await openStep(user, finish)
      expect(await screen.findByRole('heading', { name: exportHeading })).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: copyTitle }))
      await waitFor(() => expect(writeText).toHaveBeenCalledWith(POST_CONTENT_FIXTURE.title))
      // The retired publishing ACTION: no control starts one. ③'s 발행 section records an address
      // the owner published by hand, and its heading is not a control.
      expect(
        screen.queryByRole('button', { name: /^(?:발행하기|Publish)$/ }),
      ).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: /^(?:발행하기|Publish)$/ })).not.toBeInTheDocument()
      expect(calls.filter((call) => /publish/i.test(call))).toEqual([])
    },
  )
})

describe('the post language', () => {
  it('exports the frozen content provenance rather than a different current target', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/english-content', {
      user: USER,
      posts: {
        posts: [
          {
            slug: 'english-content',
            status: 'finalized',
            targetLanguage: 'ko',
            contentLanguage: 'en',
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
    })

    await openStep(user, '글 완성')
    await user.click(screen.getByRole('tab', { name: '자체 사이트' }))
    expect(screen.getByLabelText<HTMLTextAreaElement>('내보내기 결과').value).toContain(
      '<html lang="en">',
    )
    await user.click(screen.getByRole('tab', { name: '마크다운' }))
    expect(screen.getByLabelText<HTMLTextAreaElement>('내보내기 결과').value).toContain(
      '\nlanguage: en\n',
    )
  })

  it('fails closed instead of guessing when finalized content provenance is missing', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/malformed-content', {
      user: USER,
      posts: {
        posts: [
          {
            slug: 'malformed-content',
            status: 'finalized',
            contentLanguage: null,
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
    })

    await openStep(user, '글 완성')
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '글의 내용 언어 정보가 없어 내보낼 수 없어요.',
    )
    expect(screen.queryByRole('heading', { name: '내보내기' })).not.toBeInTheDocument()
  })
})

// POST-54, POST-72: ③ is what the finished post is for — 기억으로 저장, the manual export and the
// 발행 URL field at its foot — and nothing else. 말투 학습 and 문장 의견 left it with learning from
// finalized posts (VOICE r5): a finalized post teaches its voice nothing.
describe('the finish panel', () => {
  it('holds 기억으로 저장, the export and the 발행 URL field in that order, and no learning', async () => {
    const calls: string[] = []
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: { posts: [finalizedPostRow({ slug: '20260820-final' })] },
    })

    // A finalized post opens on 글 완성.
    const panel = await screen.findByRole('tabpanel', { name: '글 완성' })
    const memories = await within(panel).findByRole('button', { name: '기억으로 저장' })
    const exportHeading = within(panel).getByRole('heading', { name: '내보내기' })
    const publish = within(panel).getByRole('region', { name: '발행' })
    const FOLLOWING = Node.DOCUMENT_POSITION_FOLLOWING
    expect(memories.compareDocumentPosition(exportHeading) & FOLLOWING).toBeTruthy()
    expect(exportHeading.compareDocumentPosition(publish) & FOLLOWING).toBeTruthy()

    expect(within(panel).queryByRole('heading', { name: '말투 학습' })).not.toBeInTheDocument()
    for (const gone of ['말투 학습', '문장 의견', '수정 없이도 마음에 들어요']) {
      expect(screen.queryByRole('button', { name: gone })).not.toBeInTheDocument()
    }
    expect(calls.filter((call) => /Learn|Feedback|Validation|Comparison/.test(call))).toEqual([])
  })
})

// POST-73, POST-75, POST-77, POST-87: ③'s foot records, replaces and clears the post's Naver
// address, refuses anything else before sending, and waits for 확정.
describe('the 발행 URL field', () => {
  const slug = '20260820-seongsu'
  const finalizedPost = finalizedPostRow({ slug, title: '성수 카페' })
  const publishedPost = publishedPostRow({ slug, title: '성수 카페' })
  const statusLine = () => screen.getByRole('status', { name: '글 상태' })
  const section = () => screen.getByRole('region', { name: '발행' })
  const field = () => within(section()).getByLabelText('네이버 블로그 글 주소')
  const saveButton = () => within(section()).getByRole('button', { name: '저장' })

  function renderPost(row: FakePostRow, extra: FakePostsOptions = {}) {
    const calls: string[] = []
    const saves: string[] = []
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [row], publishedUrlSaves: saves, ...extra },
    })
    return { calls, saves }
  }

  it('publishes on a pasted address and stays on ③', async () => {
    const user = userEvent.setup()
    const { saves } = renderPost(finalizedPost)
    await user.click(await screen.findByLabelText('네이버 블로그 글 주소'))
    await user.paste('  https://blog.naver.com/alice/223000000001  ')
    await user.click(saveButton())

    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'))
    expect(saves).toEqual(['https://blog.naver.com/alice/223000000001'])
    expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true')
    expect(field()).toHaveValue('https://blog.naver.com/alice/223000000001')
  })

  it.each([
    [
      '지우기',
      async (user: ReturnType<typeof userEvent.setup>) =>
        user.click(within(section()).getByRole('button', { name: '지우기' })),
    ],
    [
      'an emptied field and 저장',
      async (user: ReturnType<typeof userEvent.setup>) => {
        await user.clear(field())
        await user.click(saveButton())
      },
    ],
  ])('clears by %s back to 확정, and ② is editable again', async (_name, clear) => {
    const user = userEvent.setup()
    const { saves } = renderPost(publishedPost)
    await screen.findByLabelText('네이버 블로그 글 주소')
    await clear(user)

    await waitFor(() => expect(statusLine()).toHaveTextContent('확정'))
    expect(saves).toEqual([''])
    expect(field()).toHaveValue('')
    await openStep(user, '글 다듬기')
    expect(
      await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }),
    ).toBeInTheDocument()
  })
})
