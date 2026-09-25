// ③ 글 완성: export, the voice-learning handoff, sentence feedback and the 발행 URL field.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { USER, openStep, resetEditorTest, stubLearningHandoff } from '@/test/editor'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
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
    // 글 완성 loads the post and the analyze selection the finalize control needs. The voice
    // profile is NOT among them any more: the empty-profile warning belongs to 글 생성, so a post
    // already past generating no longer pays for that read.
    await waitFor(() => {
      expect(calls).toEqual(expect.arrayContaining(['GetPost', 'ListModels', 'GetSelections']))
    })
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

  it('keeps a failed learning handoff across reloads so only learning can be retried', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-final'
    stubLearningHandoff({ [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1' }) })
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
      },
      jobs: {
        jobs: [
          {
            id: 'learn-1',
            kind: 'voice_learn',
            status: 'failed',
            failureReason: 'MODEL_UNAVAILABLE',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('AI 모델을 잠시 사용할 수 없어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
    expect(localStorage.getItem(key)).not.toBeNull()
    localStorage.removeItem(key)
  })

  it('removes a failed-learning retry when the current voice language is ineligible', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-mismatch'
    stubLearningHandoff({
      [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
    })
    const calls: string[] = []
    renderAppAt('/posts/20260820-mismatch', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-mismatch',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            finalizedRevision: 1n,
            contentLanguage: 'en',
            voice: {
              id: 'voice-default',
              name: '기본 말투',
              sourceLanguage: 'ko',
            },
          },
        ],
      },
      jobs: {
        jobs: [
          {
            id: 'learn-1',
            kind: 'voice_learn',
            status: 'failed',
            failureReason: 'MODEL_UNAVAILABLE',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('글과 말투의 언어가 달라 학습할 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
    expect(calls).not.toContain('RetryVoiceLearning')
    localStorage.removeItem(key)
  })

  // The one thing 글 완성 can do, and it is done: the button stays put and says so.
  it('keeps 말투 학습 disabled for a revision it has already learned from', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-final'
    stubLearningHandoff({
      [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
    })
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
      },
      jobs: { jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'done' }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    // The completed run is read back from the handoff a reload preserved, so the outcome is on
    // screen and the button cannot start the same run again.
    expect(await screen.findByText('이 글에서 말투를 배웠어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
    localStorage.removeItem(key)
  })

  it('does not let a completed handoff from an older revision hide later learning', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-final'
    stubLearningHandoff({
      [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
    })
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 2n,
            machineBaselineRevision: 2n,
            canFinalize: true,
            finalizedRevision: 2n,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
      },
      jobs: { jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'done' }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    const learn = await screen.findByRole('button', { name: '말투 학습' })
    await waitFor(() => expect(learn).toBeEnabled())
    localStorage.removeItem(key)
  })
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

// Change 16 A14: the server requires a COMPLETED voice-learning event before it accepts sentence
// feedback, and a post on 글 다듬기 is in `review` — never finalized, never learned. The control
// therefore moved to 글 완성 and is gated on the same condition the server enforces.
describe('sentence feedback', () => {
  const learnedPost = finalizedPostRow({ slug: '20260820-final' })

  it('is absent on 글 다듬기 and present on 글 완성 once the learning run has completed', async () => {
    const user = userEvent.setup()
    const key = 'postpilot:voice-learning:alice:20260820-final'
    stubLearningHandoff({
      [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
    })
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: { posts: [learnedPost] },
      jobs: { jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'done' }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('이 글에서 말투를 배웠어요.')).toBeInTheDocument()
    const feedback = screen.getByRole('button', { name: '문장 의견' })

    // What it SAYS: it teaches the voice, it does not change the post, and it names the thing
    // that does change the post.
    await user.click(feedback)
    const dialog = await screen.findByRole('dialog', { name: '어떤 점을 바꾸고 싶나요?' })
    expect(dialog).toHaveTextContent('이 의견은 말투를 가르칩니다. 이 글은 바뀌지 않아요.')
    expect(dialog).toHaveTextContent('AI 수정')
    expect(dialog).not.toHaveTextContent('이 반응만으로 새 규칙이 생기거나 활성화되지는 않습니다.')

    await user.click(within(dialog).getByRole('button', { name: '취소' }))
    await openStep(user, '글 다듬기')
    await screen.findByRole('button', { name: '제목과 요약, 태그 수정' })
    expect(screen.queryByRole('button', { name: '문장 의견' })).not.toBeInTheDocument()
    localStorage.removeItem(key)
  })

  it('is not offered on 글 완성 for a post whose learning run has not completed', async () => {
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: { posts: [learnedPost] },
    })
    await screen.findByRole('button', { name: '말투 학습' })
    expect(screen.queryByRole('button', { name: '문장 의견' })).not.toBeInTheDocument()
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

  it('is a url field at the foot of ③, typed for the keyboard that pastes an address', async () => {
    renderPost(finalizedPost)
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    for (const [name, value] of Object.entries({
      type: 'url',
      inputmode: 'url',
      autocomplete: 'url',
      autocapitalize: 'none',
      autocorrect: 'off',
      enterkeyhint: 'done',
    })) {
      expect(input).toHaveAttribute(name, value)
    }
    expect(input).toHaveValue('')
    expect(saveButton()).toBeEnabled()
    expect(within(section()).queryByRole('button', { name: '지우기' })).toBeNull()
  })

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

  it('accepts the mobile share address and shows it normalized', async () => {
    const user = userEvent.setup()
    const { saves } = renderPost(finalizedPost)
    await user.type(
      await screen.findByLabelText('네이버 블로그 글 주소'),
      'https://m.blog.naver.com/alice/7{Enter}',
    )

    await waitFor(() => expect(field()).toHaveValue('https://blog.naver.com/alice/7'))
    // What is sent is the input as typed, trimmed; the normalization is the server's answer.
    expect(saves).toEqual(['https://m.blog.naver.com/alice/7'])
    expect(statusLine()).toHaveTextContent('발행됨')
  })

  it('refuses another address in place and sends nothing', async () => {
    const user = userEvent.setup()
    const { calls } = renderPost(finalizedPost)
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    for (const bad of [
      'https://cafe.naver.com/alice/1',
      'https://blog.naver.com:443/alice/1',
      'https://user@blog.naver.com/alice/1',
    ]) {
      await user.clear(input)
      await user.type(input, bad)
      await user.click(saveButton())
      const message = await within(section()).findByRole('alert')
      expect(message).toHaveTextContent('네이버 블로그 글 주소만 저장할 수 있어요.')
      expect(input).toHaveAttribute('aria-invalid', 'true')
      expect(input.getAttribute('aria-describedby')).toContain(message.id)
    }
    expect(calls).not.toContain('SavePostPublishedUrl')
    expect(statusLine()).toHaveTextContent('확정')
  })

  it('sends nothing for an empty 저장 on a post with no address', async () => {
    const user = userEvent.setup()
    const { calls } = renderPost(finalizedPost)
    await screen.findByLabelText('네이버 블로그 글 주소')
    await user.click(saveButton())
    expect(calls).not.toContain('SavePostPublishedUrl')
  })

  it('replaces a published address and stays 발행됨', async () => {
    const user = userEvent.setup()
    const { saves } = renderPost(publishedPost)
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    expect(input).toHaveValue('https://blog.naver.com/alice/1')
    await user.clear(input)
    await user.type(input, 'https://blog.naver.com/alice/2')
    await user.click(saveButton())

    await waitFor(() => expect(field()).toHaveValue('https://blog.naver.com/alice/2'))
    expect(saves).toEqual(['https://blog.naver.com/alice/2'])
    expect(statusLine()).toHaveTextContent('발행됨')
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

  it('stays closed with its reason until the post is 확정', async () => {
    const user = userEvent.setup()
    const { calls } = renderPost({ ...finalizedPost, status: 'review', finalizedRevision: 0n })
    await openStep(user, '글 완성')
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    expect(input).toBeDisabled()
    expect(saveButton()).toBeDisabled()
    const reason = within(section()).getByText('글을 확정하면 발행 URL을 입력할 수 있어요.')
    expect(input.getAttribute('aria-describedby')).toContain(reason.id)
    expect(calls).not.toContain('SavePostPublishedUrl')
  })
})
