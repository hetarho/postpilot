// ② 글 다듬기: resumed revisions, 확정 and the content save before it, the post's measurement row,
// and a replacement take's save.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { USER, finalize, openFinalize, openStep, resetEditorTest } from '@/test/editor'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import {
  finalizedPostRow,
  type FakeDraftSave,
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
  it('resumes an active revision beside the rendered content', async () => {
    const active = {
      id: 'revision-1',
      kind: 'revise',
      status: 'running',
      stage: 'write',
      progressDone: 0,
      progressTotal: 1,
    }
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            activeJob: active,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer' },
          { providerId: 'openrouter', modelId: 'writer-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'writer' },
            candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
          },
        ],
      },
      jobs: { jobs: [active] },
    })

    expect(await screen.findByRole('heading', { name: '비 온 뒤의 제주' })).toBeInTheDocument()
    expect(screen.getByText('작성 중')).toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청을 입력하세요')).toBeDisabled()
  })

  it('does not offer a no-op retry when a resumed revision fails', async () => {
    const resumed = {
      id: 'revision-1',
      kind: 'revise',
      status: 'running',
      stage: 'write',
      progressDone: 0,
      progressTotal: 1,
    }
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            activeJob: resumed,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer' },
          { providerId: 'openrouter', modelId: 'writer-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'writer' },
            candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
          },
        ],
      },
      jobs: {
        jobs: [{ ...resumed, status: 'failed', failureReason: 'MODEL_UNAVAILABLE' }],
      },
    })

    expect(await screen.findByText('AI 모델을 잠시 사용할 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청을 입력하세요')).toBeEnabled()
  })

  // A8 (client half): 확정 copies the AI title into `posts.title`, and the editor still holds the
  // 가제 in state where `useAutosave` would write it straight back on the next keystroke.
  it('re-seeds the local 가제 from the confirmed title before another save can queue', async () => {
    const draftSaves: FakeDraftSave[] = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        draftSaves,
        posts: [
          {
            slug: '20260820-final',
            status: 'review',
            title: '가제',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
    })

    await finalize(user)

    await waitFor(() => expect(calls).toContain('FinalizePost'))
    await openStep(user, '글 생성')
    await waitFor(() =>
      expect(screen.getByLabelText('제목')).toHaveValue(POST_CONTENT_FIXTURE.title),
    )
    // The next ordinary save must carry the confirmed title, not the placeholder — which is what
    // the list row is read from (A8/A9).
    await user.type(screen.getByLabelText('메모'), '뒷이야기')
    await waitFor(() => expect(draftSaves).toHaveLength(1), { timeout: 4_000 })
    await user.click(screen.getByRole('link', { name: '글 목록' }))
    expect(
      await screen.findByRole('link', { name: new RegExp(POST_CONTENT_FIXTURE.title) }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /가제/ })).not.toBeInTheDocument()
  })

  it('finalizes without an analyze model or learning call', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
    })
    // 확정하기 ends 글 다듬기, where the post opens, and offers both ways out.
    const panel = await openFinalize(user)
    const only = within(panel).getByRole('button', { name: '확정' })
    expect(only).toBeEnabled()
    expect(within(panel).getByRole('button', { name: '확정하고 말투 학습' })).toBeDisabled()
    await user.click(only)
    await waitFor(() => expect(calls).toContain('FinalizePost'))
    expect(calls).not.toContain('LearnFromFinalizedPost')
    // Confirming carries the user to 글 완성, whose own action is learning — and this account has
    // no analyze model, so it is offered as disabled rather than hidden.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true'),
    )
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeDisabled()
  })

  it('keeps the post finalized when explicit learning fails', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
      voice: { learningFails: true },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })
    const panel = await openFinalize(user)
    const combined = within(panel).getByRole('button', { name: '확정하고 말투 학습' })
    await waitFor(() => expect(combined).toBeEnabled())
    await user.click(combined)
    await waitFor(() =>
      expect(calls).toEqual(expect.arrayContaining(['FinalizePost', 'LearnFromFinalizedPost'])),
    )
    // The failure is reported on 글 완성 — where the learning run lands — and the finalize it
    // followed still stands, so only learning is retried.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true'),
    )
    expect(screen.getByText(/글은 확정됐지만 말투 학습은 시작하지 못했어요/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeEnabled()
  })

  it('returns a finalized post to review after the first changed content save', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        posts: [finalizedPostRow({ slug: '20260820-final', images: POST_IMAGES_FIXTURE })],
      },
    })

    // A finalized post opens on 글 완성.
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeInTheDocument()

    await openStep(user, '글 다듬기')
    await user.click(await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }))
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })

    // Back in review, so 확정 is offered again where it belongs, and 글 완성 can no longer learn
    // from a revision the post has moved past.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
        'aria-selected',
        'true',
      ),
    )
    expect(await screen.findByRole('button', { name: '확정하기' })).toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
  })
})

describe('the step split and the content save', () => {
  const reviewPost = {
    slug: '20260820-final',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }

  // 확정 sits at the end of the step the block editor lives on, so it flushes that editor's
  // pending save through its live ref — and the queue behind it — before it names a revision.
  it('saves an edited block before finalizing it', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', { user: USER, calls, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '확정 직전에 고친 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await finalize(user)

    await waitFor(() => expect(calls).toContain('FinalizePost'))
    // The content save is ahead of the finalize, so the finalized revision includes the edit.
    expect(calls.indexOf('SavePostContent')).toBeGreaterThan(-1)
    expect(calls.indexOf('SavePostContent')).toBeLessThan(calls.indexOf('FinalizePost'))
  })
})

// QUAL-36, POST-83: ② carries this post's own M2, M3 and M4 above the article it measures, read at
// the revision on screen.
describe('the post measurement row', () => {
  const HEADING = '이 글의 측정값'
  const REVIEW = {
    slug: '20260820-measured',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }
  const MEASURED = {
    [REVIEW.slug]: [
      {
        metric: 'cross_post_phrases' as const,
        verdict: 'within_band' as const,
        minimum: 3,
        publishedCount: 4,
        values: { share: 0.05, shareWarnAbove: 0.1 },
      },
      {
        metric: 'in_post_repetition' as const,
        verdict: 'over_band' as const,
        values: {
          repetitionShare: 0.12,
          titleRelevance: 0.6,
          repetitionShareWarnAbove: 0.08,
          titleRelevanceWarnBelow: 0.5,
        },
      },
      {
        metric: 'composition' as const,
        verdict: 'within_band' as const,
        values: {
          charCount: 820,
          photoCount: 2,
          distinctBlockTypes: 3,
          averageSentenceLength: 12.5,
          distinctBlockTypesWarnAtOrBelow: 2,
        },
      },
    ],
  }
  const measurementReads = (calls: string[]) =>
    calls.filter((call) => call === 'GetPostMeasurement').length

  it('sits above the article on ②', async () => {
    // English content under the Korean interface: a sentence is counted in the content's words.
    renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      posts: { posts: [{ ...REVIEW, contentLanguage: 'en' }] },
      quality: { measurements: MEASURED },
    })

    const region = await screen.findByRole('region', { name: HEADING })
    // Directly above the article, under 글 다듬기 and its save status.
    expect(region.nextElementSibling).toBe(screen.getByRole('article', { name: '생성된 글' }))
    expect(screen.getByRole('region', { name: '글 다듬기' })).toContainElement(region)
    expect(await within(region).findByText('본문 명사 중 가장 잦은 명사')).toBeInTheDocument()
    expect(within(region).getByText('주의')).toBeInTheDocument()
    expect(within(region).getAllByText('양호')).toHaveLength(2)
    expect(within(region).getByText('12.5단어')).toBeInTheDocument()
  })

  it('refetches after a block save', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [REVIEW] },
      quality: { measurements: MEASURED },
    })
    const region = await screen.findByRole('region', { name: HEADING })
    await within(region).findByText('글자 수')
    expect(measurementReads(calls)).toBe(1)

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '측정을 다시 부르는 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    // The save moves the revision the row reads, so the next revision is read — once the save
    // has landed, and not before.
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
    await waitFor(() => expect(measurementReads(calls)).toBeGreaterThan(1))
    // Reading at the revision the save produced is the entity's rule (QUAL-3), pinned by
    // PostMeasurementRow.test.tsx.
    expect(calls.indexOf('SavePostContent')).toBeLessThan(calls.lastIndexOf('GetPostMeasurement'))
    // The previous reading stays on screen while the next one loads: never the loading line again.
    expect(within(region).queryByText('측정하는 중이에요.')).toBeNull()
  })

  it('is absent while ② has no draft', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [{ slug: REVIEW.slug, title: '제주' }] },
      quality: { measurements: MEASURED },
    })

    await openStep(user, '글 다듬기')
    expect(await screen.findByText(/아직 다듬을 글이 없어요/)).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: HEADING })).toBeNull()
    expect(measurementReads(calls)).toBe(0)
  })
})

// POST-79, POST-80: ② marks where the last write's candidates still stand, and taking a phrase is
// an ordinary content save that records nothing about where the words came from.
describe('the replacement marks', () => {
  const SLUG = '20260820-rain'
  const CANDIDATES: NonNullable<FakePostRow['replacementCandidates']> = [
    { surface: 'title', index: 0, source: '제주', phrases: ['제주도', '제주 바다'] },
    // '여행' would duplicate tag 2, so only '산책로' is offered.
    { surface: 'tag', index: 1, source: '산책', phrases: ['산책로', '여행'] },
    {
      surface: 'body',
      index: 0,
      source: '기다렸다',
      phrases: ['기다린다', '기다려 본다', '기다리고 있었다', '기다렸어요'],
    },
    { surface: 'body', index: 0, source: '비가', phrases: ['빗줄기가'] },
    { surface: 'body', index: 2, source: '바닷가로', phrases: ['해변으로'] },
  ]
  const finalized = finalizedPostRow({
    slug: SLUG,
    images: POST_IMAGES_FIXTURE,
    replacementCandidates: CANDIDATES,
  })
  const article = () => within(screen.getByRole('article', { name: '생성된 글' }))

  function renderMarks(row: FakePostRow = finalized) {
    const calls: string[] = []
    const contentSaves: NonNullable<FakePostsOptions['contentSaves']> = []
    const view = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, contentSaves, posts: [row] },
    })
    return { calls, contentSaves, view }
  }

  async function openRefine(user: ReturnType<typeof userEvent.setup>) {
    await openStep(user, '글 다듬기')
    await screen.findByRole('article', { name: '생성된 글' })
  }

  it('saves a take with the expected revision and returns 확정 to 검토', async () => {
    const user = userEvent.setup()
    const { contentSaves } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    await user.click(await screen.findByRole('button', { name: '‘기다려 본다’(으)로 바꾸기' }))

    await waitFor(() => expect(contentSaves).toHaveLength(1), { timeout: 4_000 })
    expect(contentSaves[0].slug).toBe(SLUG)
    expect(contentSaves[0].expectedRevision).toBe(1n)
    // The same save spends the candidate it took, by its index in the stored list (POST-79).
    expect(contentSaves[0].takenCandidates).toEqual([2])
    expect(contentSaves[0].content.blocks[0].content).toBe('비가 그치기를 기다려 본다.')
    // The rest of the content went as the editor held it.
    expect(contentSaves[0].content.title).toBe(POST_CONTENT_FIXTURE.title)
    expect(contentSaves[0].content.tags).toEqual(POST_CONTENT_FIXTURE.tags)
    // Focus lands on the pencil of the block that held the text, never on <body>.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '1번째 블록 수정' })).toHaveFocus(),
    )
    await waitFor(() =>
      expect(screen.getByRole('status', { name: '글 상태' })).toHaveTextContent('검토'),
    )
    expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute('aria-selected', 'true')
  })
})
