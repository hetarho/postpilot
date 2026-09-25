// ② 글 다듬기: the draft read first, the block editor and its save, 확정, the post's measurement
// row and the replacement marks.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { USER, finalize, openFinalize, openStep, resetEditorTest } from '@/test/editor'
import {
  POST_CONTENT_FIXTURE,
  POST_CONTENT_WITH_VIDEO_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
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

describe('the draft read-first', () => {
  const reviewPost = {
    slug: '20260820-jeju',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
  }

  // Change 05 A7 / A10.
  it('renders the draft as prose with no form control until a block is opened', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', { user: USER, posts: { posts: [reviewPost] } })

    const draft = await screen.findByRole('article', { name: '생성된 글' })
    expect(within(draft).queryByRole('textbox')).not.toBeInTheDocument()
    expect(within(draft).getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    expect(screen.getByLabelText('1번째 블록 내용')).toHaveValue(
      POST_CONTENT_FIXTURE.blocks[0].content,
    )
  })

  // Change 05 A8: cancel restores the value the block had when its editor opened.
  it('restores a cancelled block and keeps a saved one as prose', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', { user: USER, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '고쳐 쓴 문단')
    await user.click(screen.getByRole('button', { name: '취소' }))

    expect(screen.queryByLabelText('1번째 블록 내용')).not.toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const reopened = screen.getByLabelText('1번째 블록 내용')
    await user.clear(reopened)
    await user.type(reopened, '확정한 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    expect(screen.queryByLabelText('1번째 블록 내용')).not.toBeInTheDocument()
    expect(screen.getByText('확정한 문단')).toBeInTheDocument()
  })

  // Change 05 A9: the editing UI keeps every capability it had.
  it('keeps add, delete and move available from a block editor', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', { user: USER, calls, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '2번째 블록 수정' }))
    expect(screen.getByRole('button', { name: '2번째 블록 위로' })).toBeEnabled()
    expect(screen.getByLabelText('2번째 블록 종류')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '삭제' }))
    expect(screen.queryByLabelText('2번째 블록 종류')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '문단 추가' }))
    expect(await screen.findByText('새 문단')).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
  })

  // VIDEO-2: a VIDEO block carries the IMAGE fields and none of its own, so the same three
  // controls edit it — only the list it picks from differs, and it is offered only when the
  // post actually has a clip.
  it('edits a VIDEO block with the attached-video list, alt and caption', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-video', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-video',
            status: 'review',
            content: POST_CONTENT_WITH_VIDEO_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            videos: [{ id: 'video-1', filename: 'clip.mp4' }],
            contentRevision: 1n,
            machineBaselineRevision: 1n,
          },
        ],
      },
    })

    const blockIndex = POST_CONTENT_WITH_VIDEO_FIXTURE.blocks.length - 1
    await user.click(await screen.findByRole('button', { name: `${blockIndex + 1}번째 블록 수정` }))
    expect(screen.getByText('첨부 영상')).toBeInTheDocument()
    const caption = screen.getByPlaceholderText('캡션 (선택)')
    await user.type(caption, '파도')
    await user.click(screen.getByRole('button', { name: '저장' }))
    expect(await screen.findByText('파도')).toBeInTheDocument()
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

  // 취소 restores the block it opened on, so moving must close the editor rather than leave a
  // snapshot pointed at whichever block shifted into that slot.
  it('closes a block editor when the block moves', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', { user: USER, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '2번째 블록 수정' }))
    await user.click(screen.getByRole('button', { name: '2번째 블록 위로' }))

    expect(screen.queryByRole('button', { name: '취소' })).not.toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[1].content)).toBeInTheDocument()
  })

  // The header editor owns the title, summary and tags — cancelling it must not revert a block.
  it('keeps a block edit made while the header editor was open', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', { user: USER, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }))
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const block = screen.getByLabelText('1번째 블록 내용')
    await user.clear(block)
    await user.type(block, '유지되어야 하는 문단')
    await user.click(within(block.closest('article')!).getByRole('button', { name: '저장' }))

    // 취소 on the header restores its three fields only.
    await user.click(screen.getAllByRole('button', { name: '취소' })[0])

    expect(screen.getByText('유지되어야 하는 문단')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: POST_CONTENT_FIXTURE.title })).toBeInTheDocument()
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

  it('marks the title, a tag and a TEXT block', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    const heading = article().getByRole('heading', { level: 3 })
    const title = within(heading)
    expect(title.getByRole('button', { name: '제주' })).toHaveAttribute('aria-haspopup', 'dialog')
    // The text around a mark is kept whole.
    expect(heading).toHaveTextContent(/^비 온 뒤의 제주$/)
    expect(article().getByRole('button', { name: '비가' }).parentElement).toHaveTextContent(
      /^비가 그치기를 기다렸다\.$/,
    )
    const tags = within(article().getByRole('list', { name: '태그' }))
    // A chip stands alone, so its mark grows to the pointer floor under a coarse pointer; a mark in
    // a sentence takes WCAG 2.5.8's inline exception (THEME-23).
    expect(tags.getByRole('button', { name: '산책' })).toHaveClass(
      'pointer-coarse:min-h-11',
      'pointer-coarse:min-w-11',
    )
    expect(article().getByRole('button', { name: '기다렸다' })).not.toHaveClass(
      'pointer-coarse:min-h-11',
    )
    // The tag chip with no candidate stays plain text.
    expect(tags.queryByRole('button', { name: '제주' })).toBeNull()
    expect(article().getByRole('button', { name: '기다렸다' })).toBeInTheDocument()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
  })

  it('offers at most three phrases and no duplicate tag', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    const panel = within(await screen.findByRole('dialog', { name: '‘기다렸다’ 바꿔 쓰기' }))
    expect(panel.getAllByRole('button').map((button) => button.textContent)).toEqual([
      '기다린다',
      '기다려 본다',
      '기다리고 있었다',
    ])
    expect(panel.getByRole('button', { name: '‘기다린다’(으)로 바꾸기' })).toBeInTheDocument()
    expect(panel.getByText('‘기다렸다’ 대신 쓸 수 있는 표현')).toBeInTheDocument()
    // What was observed about the phrases, and nothing about what they gain.
    expect(
      panel.getByText('네이버 검색 결과의 제목과 설명에 자주 나온 표현이에요.'),
    ).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await user.click(article().getByRole('button', { name: '산책' }))
    const tagPanel = within(await screen.findByRole('dialog', { name: '‘산책’ 바꿔 쓰기' }))
    expect(tagPanel.getAllByRole('button').map((button) => button.textContent)).toEqual(['산책로'])
  })

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

  // POST-79: a taken candidate is spent. Its mark never comes back — not even where the phrase
  // holds its source, as 제주도 holds 제주 — and every other mark stays. Focus goes to the header's
  // pencil.
  it('spends a taken candidate, even where its phrase holds the source', async () => {
    const user = userEvent.setup()
    const { contentSaves, view } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '제주' }))
    await user.click(await screen.findByRole('button', { name: '‘제주도’(으)로 바꾸기' }))

    expect(screen.queryByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeNull()
    const spent = () => {
      const heading = article().getByRole('heading', { level: 3 })
      expect(heading).toHaveTextContent(/^비 온 뒤의 제주도$/)
      expect(within(heading).queryByRole('button', { name: /제주/ })).toBeNull()
      for (const neighbour of ['산책', '기다렸다', '비가'])
        expect(article().getByRole('button', { name: neighbour })).toBeInTheDocument()
    }
    spent()
    await waitFor(() => expect(contentSaves).toHaveLength(1), { timeout: 4_000 })
    expect(contentSaves[0].content.title).toBe('비 온 뒤의 제주도')
    expect(contentSaves[0].takenCandidates).toEqual([0])
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '제목과 요약, 태그 수정' })).toHaveFocus(),
    )
    spent()

    view.unmount()
    renderAppAt(`/posts/${SLUG}`, { transport: view.transport })
    await openRefine(user)
    spent()
  })

  it('puts focus on the pencil of the block the take changed', async () => {
    const user = userEvent.setup()
    const { contentSaves } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '바닷가로' }))
    await user.click(await screen.findByRole('button', { name: '‘해변으로’(으)로 바꾸기' }))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: '3번째 블록 수정' })).toHaveFocus(),
    )
    await waitFor(() => expect(contentSaves).toHaveLength(1), { timeout: 4_000 })
    expect(contentSaves[0].content.blocks[2].content).toBe('해변으로')
  })

  it('keeps a neighbouring mark after a take', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    await user.click(await screen.findByRole('button', { name: '‘기다린다’(으)로 바꾸기' }))

    await waitFor(() => expect(article().queryByRole('button', { name: '기다렸다' })).toBeNull())
    expect(article().getByText(/기다린다\./)).toBeInTheDocument()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
    expect(article().getByRole('button', { name: '제주' })).toBeInTheDocument()
  })

  it('drops a mark whose source was edited away', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    // An open editor is plain fields: no mark stands in the block it edits.
    expect(article().queryByRole('button', { name: '비가' })).toBeNull()
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '비가 그쳤다.')
    await user.click(screen.getByRole('button', { name: '저장' }))

    expect(article().queryByRole('button', { name: '기다렸다' })).toBeNull()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
  })

  it('sends nothing for an ignored mark', async () => {
    const user = userEvent.setup()
    const { calls } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '제주' }))
    expect(await screen.findByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeNull()

    // Past the content debounce: opening and closing a mark is no edit (POST-79).
    await new Promise((resolve) => setTimeout(resolve, 1_500))
    expect(calls).not.toContain('SavePostContent')
  })
})
