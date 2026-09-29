// ② 글 다듬기: resumed revisions, 확정 and the content save before it, the measurement row and the
// fingerprint row.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoFingerprintFacetUnit, ProtoFingerprintItem, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { USER, finalize, openStep, resetEditorTest } from '@/test/editor'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import type { FakeGenerationStart } from '@/test/jobs'
import { finalizedPostRow, type FakeDraftSave } from '@/test/posts'
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

  // POST-13: an IMAGE block naming a photo the post no longer has refuses the finalize, and ③
  // says how many such places remain and what to do about them.
  it('says how many image blocks name a detached photo and finalizes none of it', async () => {
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
            title: '가제',
            content: POST_CONTENT_FIXTURE,
            // IMG_2 was deleted after the text was written; its IMAGE block still names it.
            images: [POST_IMAGES_FIXTURE[0]!],
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
    })

    // The editor names the problem where it is fixed, with the count, before any finalize.
    expect(await screen.findByText(/사진이 없는 자리가 1곳 남아 있어요/)).toBeInTheDocument()
    await finalize(user)
    // Content that cannot save cannot be finalized: nothing reaches the server.
    await new Promise((resolve) => setTimeout(resolve, 200))
    expect(calls).not.toContain('FinalizePost')
  })

  // POST-56: 확정하기 finalizes at once — no popover or modal between the press and the run — and
  // needs no analyze model, because a finalize teaches no voice anything. It carries the user to
  // 글 완성, which holds 기억으로 저장, the export and the address field, and no learning control.
  it('finalizes at once on 확정하기 and lands on 글 완성', async () => {
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
    const trigger = await screen.findByRole('button', { name: '확정하기' })
    // A plain button, not a trigger for a surface.
    expect(trigger).not.toHaveAttribute('aria-expanded')
    expect(trigger).not.toHaveAttribute('aria-haspopup')
    await user.click(trigger)

    await waitFor(() => expect(calls).toContain('FinalizePost'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true'),
    )
    expect(await screen.findByRole('button', { name: '기억으로 저장' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '말투 학습' })).not.toBeInTheDocument()
    expect(calls.filter((call) => call.includes('Learn'))).toEqual([])
  })

  // ② used to warn when the draft's language differed from its voice's samples — a notice that
  // only ever said why the post could not teach that voice, and a finalized post teaches no voice
  // now. The notice the post's own target and content languages raise is a different one.
  it('shows no voice/content language notice on ②', async () => {
    renderAppAt('/posts/20260820-final', {
      user: USER,
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
            contentLanguage: 'ko',
            voice: { id: 'voice-english', name: '영어 말투' },
          },
        ],
      },
    })

    const panel = await screen.findByRole('tabpanel', { name: '글 다듬기' })
    expect(
      await within(panel).findByRole('button', { name: '제목과 요약, 태그 수정' }),
    ).toBeInTheDocument()
    expect(screen.queryByText(/말투의 언어|언어가 달라/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '확정하기' })).toBeEnabled()
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
    expect(await screen.findByRole('button', { name: '기억으로 저장' })).toBeInTheDocument()

    await openStep(user, '글 다듬기')
    await user.click(await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }))
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })

    // Back in review, so 확정하기 is offered again where it belongs, and nowhere else.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
        'aria-selected',
        'true',
      ),
    )
    expect(await screen.findByRole('button', { name: '확정하기' })).toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '기억으로 저장' })).toBeInTheDocument()
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

// POST-102: under the measurements, ② shows this post's fingerprint beside its voice's, read at the
// revision on screen; a post with 말투 없음 shows none and asks for none.
describe('the fingerprint row', () => {
  const HEADING = '말투 지문'
  const POST = {
    slug: '20260929-fingerprint',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }
  const share = (value: number) => ({ value: { case: 'number' as const, value } })
  const FINGERPRINT = {
    [POST.slug]: {
      applicable: true,
      revision: 1n,
      items: [
        {
          item: ProtoFingerprintItem.ENDINGS,
          distance: 0.9,
          headline: '해요',
          facets: [
            {
              key: '다',
              unit: ProtoFingerprintFacetUnit.SHARE,
              voice: share(0.05),
              text: share(0.95),
            },
            {
              key: '해요',
              unit: ProtoFingerprintFacetUnit.SHARE,
              voice: share(0.92),
              text: share(0),
            },
          ],
        },
        { item: ProtoFingerprintItem.EMOJI, unknown: true },
      ],
    },
  }

  it('sits under the measurements for a post with a made voice', async () => {
    renderAppAt(`/posts/${POST.slug}`, {
      user: USER,
      posts: { posts: [POST] },
      voice: { postFingerprints: FINGERPRINT },
    })

    const region = await screen.findByRole('region', { name: HEADING })
    expect(screen.getByRole('region', { name: '이 글의 측정값' }).nextElementSibling).toBe(region)
    expect(region.nextElementSibling).toBe(screen.getByRole('article', { name: '생성된 글' }))
    const rows = within(region).getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent("문장 끝'~해요' 내 말투 92% · 이 글 0%")
    expect(rows[1]).toHaveTextContent('이모지와 자모알 수 없음')
    // The post's own reading: nothing about its template.
    expect(within(region).queryByText(/템플릿/)).toBeNull()
  })

  it('is absent for a post with 말투 없음', async () => {
    const reads: string[] = []
    renderAppAt(`/posts/${POST.slug}`, {
      user: USER,
      posts: { posts: [{ ...POST, voice: null }] },
      voice: { postFingerprints: FINGERPRINT, postFingerprintReads: reads },
    })

    await screen.findByRole('region', { name: '이 글의 측정값' })
    expect(screen.queryByRole('region', { name: HEADING })).toBeNull()
    expect(reads).toEqual([])
  })

  it('reads again at the revision a save produces', async () => {
    const calls: string[] = []
    const reads: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${POST.slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [POST] },
      voice: { postFingerprints: FINGERPRINT, postFingerprintReads: reads },
    })
    const region = await screen.findByRole('region', { name: HEADING })
    expect(reads).toHaveLength(1)

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '지문을 다시 세는 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
    await waitFor(() => expect(reads).toHaveLength(2))
    // The previous reading stays while the next one is counted.
    expect(region).toBeInTheDocument()
  })
})

// POST-95, POST-96: a draft that holds a storyline opens ② on its space, open while there is no
// post yet, and the owner's text edit saves through the draft save as the whole paragraph list.
describe('the storyline space', () => {
  const slug = '20260828-storyline'
  const storyline = {
    paragraphs: [
      { text: '가게 앞을 보여줍니다.', files: ['IMG_1.jpg'] },
      { text: '커피를 이야기합니다.', files: [] },
    ],
  }

  it('shows the storyline above a waiting draft and saves a text edit', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      posts: {
        posts: [{ slug, title: '성수', images: POST_IMAGES_FIXTURE.slice(0, 1), storyline }],
        draftSaves,
      },
    })

    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
        'aria-selected',
        'true',
      ),
    )
    expect(screen.getByRole('button', { name: '스토리라인' })).toHaveAttribute(
      'aria-expanded',
      'true',
    )
    expect(screen.getByText('이 스토리로 글을 쓰면 여기에 글이 나와요')).toBeInTheDocument()

    const second = within(screen.getByRole('listitem', { name: '2번째 문단' }))
    await user.click(second.getByRole('button', { name: '2번째 문단 고치기' }))
    await user.type(second.getByRole('textbox', { name: '2번째 문단' }), ' 라떼')
    await waitFor(() => expect(draftSaves.some((save) => save.storyline)).toBe(true), {
      timeout: 4_000,
    })
    expect(draftSaves.find((save) => save.storyline)?.storyline).toEqual([
      storyline.paragraphs[0],
      { text: '커피를 이야기합니다. 라떼', files: [] },
    ])
  })

  it('is read-only while a job targets the post', async () => {
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      posts: {
        posts: [
          {
            slug,
            storyline,
            activeJob: { id: 'job-1', kind: 'storyline', status: 'running', stage: 'storyline' },
          },
        ],
      },
      jobs: { jobs: [{ id: 'job-1', kind: 'storyline', status: 'running', stage: 'storyline' }] },
    })
    expect(await screen.findByRole('listitem', { name: '1번째 문단' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '1번째 문단 고치기' })).not.toBeInTheDocument()
  })

  // POST-98: over a post edited by hand, 이 스토리로 다시 쓰기 asks first, then saves and writes along
  // the storyline; ②'s own dock keeps the revision composer and 확정하기.
  it('rewrites from the storyline after asking, over a post edited by hand', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const starts: FakeGenerationStart[] = []
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug,
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 3n,
            machineBaselineRevision: 2n,
            storyline,
          },
        ],
      },
      jobs: { starts },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'observer', vision: true },
          { providerId: 'openrouter', modelId: 'writer' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'observer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
        ],
      },
    })
    const rewrite = await screen.findByRole('button', { name: '이 스토리로 다시 쓰기' })
    await waitFor(() => expect(rewrite).toBeEnabled())
    // Closed over a post that already has content, and the dock is still ②'s own.
    expect(screen.getByRole('button', { name: '스토리라인' })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
    const dock = screen.getByLabelText('글 작업')
    expect(within(dock).getByRole('button', { name: /확정/ })).toBeInTheDocument()

    await user.click(rewrite)
    const dialog = await screen.findByRole('dialog', { name: '이 스토리로 다시 쓸까요?' })
    expect(calls).not.toContain('StartGeneration')
    await user.click(within(dialog).getByRole('button', { name: '다시 쓰기' }))
    await waitFor(() => expect(starts).toHaveLength(1))
    expect(starts[0]).toMatchObject({ postSlug: slug, fromStoryline: true })
  })

  it('shows no space for a post written before storylines existed', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [{ slug: '20260820-jeju', status: 'review', content: POST_CONTENT_FIXTURE }],
      },
    })
    expect(await screen.findByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    expect(screen.queryByRole('button', { name: '스토리라인' })).not.toBeInTheDocument()
  })
})
