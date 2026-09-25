// ① 글 생성: starting a run, the re-observation picker, the photos and the writing brief's run
// options and quality rows.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ExperimentOrigin, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  BRIEF_TRIGGER,
  M1_OVER_BAND,
  USER,
  briefField,
  openBrief,
  resetEditorTest,
} from '@/test/editor'
import {
  OBSERVATION_FIXTURE,
  POST_CONTENT_FIXTURE,
  OBSERVATIONS_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import type { FakeWriteExperimentStart } from '@/test/experiments'
import type { FakeGenerationStart } from '@/test/jobs'
import type { FakeDraftSave, FakeOptionsSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('opening a post', () => {
  it('resumes polling the active job exposed by the post', async () => {
    const calls: string[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            activeJob: {
              id: 'job-1',
              status: 'running',
              stage: 'observe',
              progressDone: 2,
              progressTotal: 4,
            },
          },
        ],
      },
      jobs: {
        jobs: [
          {
            id: 'job-1',
            status: 'running',
            stage: 'observe',
            progressDone: 2,
            progressTotal: 4,
          },
        ],
      },
    })

    // The stage is NAMED on the page-top status line and its numbers are the bar's value —
    // never spelled out as prose in the dock (change 15).
    expect(await screen.findByText('사진 관찰 중')).toBeInTheDocument()
    const bar = screen.getByRole('progressbar', { name: '작업 진행률' })
    expect(bar).toHaveAttribute('aria-valuenow', '2')
    expect(bar).toHaveAttribute('aria-valuemax', '4')
    expect(screen.queryByText(/사진 2\/4/)).not.toBeInTheDocument()
    expect(calls).toContain('GetGeneration')
  })

  it('refreshes observations while running and renders the review draft after completion', async () => {
    const slug = '20260820-jeju'
    const image = { id: 'img-1', filename: 'IMG_1.jpg' }
    const active = {
      id: 'job-1',
      status: 'running',
      stage: 'observe',
      progressDone: 0,
      progressTotal: 1,
    }
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      posts: {
        posts: [{ slug, images: [image], activeJob: active }],
        getSequence: [
          { slug, images: [image], activeJob: active },
          { slug, images: [image], activeJob: active },
          {
            slug,
            images: [image],
            observations: [OBSERVATION_FIXTURE],
            activeJob: { ...active, progressDone: 1 },
          },
          {
            slug,
            status: 'review',
            images: [image],
            observations: [OBSERVATION_FIXTURE],
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
      jobs: {
        sequence: [
          { ...active, progressDone: 1 },
          { id: 'job-1', status: 'done', stage: 'write', progressDone: 1, progressTotal: 1 },
        ],
      },
    })

    expect(await screen.findByText('비가 그친 바닷가')).toBeInTheDocument()
    expect(screen.queryByText('관찰 대기')).not.toBeInTheDocument()
    expect(await screen.findByText('검토', {}, { timeout: 4_000 })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '비 온 뒤의 제주' })).toBeInTheDocument()
  })

  it('keeps the empty-profile warning non-blocking for zero-photo generation', async () => {
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      posts: { posts: [{ slug: '20260820-memo', memo: '사진 없는 메모' }] },
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
    })

    const user = userEvent.setup()
    expect(await screen.findByText(/문체 프로필이 비어 있어요/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '생성' })).toBeEnabled())
    expect(screen.getByLabelText('글 작업').previousElementSibling).toHaveClass('mt-auto', 'h-6')

    const brief = await openBrief(user)
    expect(within(brief).getByText('사진이 없어 관찰 모델은 필요하지 않아요.')).toBeInTheDocument()
  })

  it('flushes the newest memo before it starts generation', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-memo', memo: '처음 메모' }] },
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
    })

    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.type(screen.getByLabelText('메모'), ' + 최신 내용')
    await user.click(generate)

    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(calls.filter((call) => call === 'SavePostDraft' || call === 'StartGeneration')).toEqual([
      'SavePostDraft',
      'StartGeneration',
    ])
  })

  // Change 21: a post that has already been observed decides what to re-observe BEFORE the
  // enqueue, and confirming the picker untouched reuses everything.
  it('routes generation through the re-observation picker and freezes the confirmed set', async () => {
    const starts: FakeGenerationStart[] = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      jobs: { starts },
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            images: POST_IMAGES_FIXTURE,
            observations: OBSERVATIONS_FIXTURE,
          },
        ],
      },
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

    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    // An unsaved edit, so the draft flush on the start path is a real request whose ORDER
    // against the picker is observable.
    await user.type(screen.getByLabelText('메모'), ' + 최신 내용')
    await user.click(generate)

    // The picker opens instead of enqueueing, and `beforeStart` has NOT run: a cancelled picker
    // must not have forced a draft save.
    const picker = await screen.findByRole('dialog', { name: '다시 관찰할 사진 선택' })
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('SavePostDraft')
    await user.click(within(picker).getByRole('button', { name: '취소' }))
    expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
    expect(calls).not.toContain('StartGeneration')

    // Reopened, one photo checked, confirmed: the frozen set is exactly that photo, and the
    // draft save now sits on the confirm path, before the RPC that consumes it.
    await user.click(screen.getByRole('button', { name: '생성' }))
    await user.click(await screen.findByRole('checkbox', { name: 'IMG_2.jpg 다시 관찰' }))
    await user.click(screen.getByRole('button', { name: '이대로 시작' }))
    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(starts).toHaveLength(1)
    expect(starts[0].reobserveFiles).toEqual(['IMG_2.jpg'])
    expect(calls.indexOf('SavePostDraft')).toBeGreaterThanOrEqual(0)
    expect(calls.indexOf('SavePostDraft')).toBeLessThan(calls.indexOf('StartGeneration'))
  })

  it('starts an explicit A/B comparison with the configured pair and optional target', async () => {
    const starts: FakeWriteExperimentStart[] = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-memo' }] },
      experiments: { starts },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'active' },
          { providerId: 'openrouter', modelId: 'candidate-a' },
          { providerId: 'openrouter', modelId: 'candidate-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'active' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'candidate-a' },
            candidateB: { providerId: 'openrouter', modelId: 'candidate-b' },
          },
        ],
      },
    })
    const brief = await openBrief(user)
    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    // The box arrives with the default already in it, so this is a replacement, not an entry.
    await user.clear(within(brief).getByLabelText('목표 글자 수'))
    await user.type(within(brief).getByLabelText('목표 글자 수'), '750')
    await user.click(within(brief).getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('SavePostGenerationOptions'))
    const compare = screen.getByRole('button', { name: 'A/B 비교' })
    await waitFor(() => expect(compare).toBeEnabled())
    await user.click(compare)
    await waitFor(() => expect(calls).toContain('StartWriteExperiment'))
    expect(starts).toEqual([
      {
        postSlug: '20260820-memo',
        // Started from the editor, so its verdict will apply the winner to this very post.
        origin: ExperimentOrigin.EDITOR,
        observeModel: undefined,
        modelA: { providerId: 'openrouter', modelId: 'candidate-a' },
        modelB: { providerId: 'openrouter', modelId: 'candidate-b' },
        targetLength: 750,
      },
    ])
    expect(calls).not.toContain('StartGeneration')
  })
})

// POST-89: 분야 and 기억 사용 are run options, so they live in the writing brief with the other
// three and save together by its 저장 — none of them is ①'s material (POST-54).
describe('the brief run options', () => {
  const SLUG = '20260301-jeju'
  const WITH_FIELDS = [
    {
      id: 'template-review',
      name: '정보성 식당 리뷰',
      body: '<write>인트로</write>\n<ask label="방문일"/>\n<ask label="총평 별점">별점과 총평</ask>',
    },
  ]
  const POST_TEMPLATES = [{ id: 'template-review', name: '정보성 식당 리뷰' }]
  const following = (first: Node, second: Node) =>
    Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING)
  const m1 = () => screen.findByRole('checkbox', { name: /^제목 도배율 42%/ })
  const briefClosed = () =>
    waitFor(() =>
      expect(screen.getByRole('button', { name: BRIEF_TRIGGER })).toHaveAttribute(
        'aria-expanded',
        'false',
      ),
    )

  it('keeps 분야 and 기억 사용 out of ①, after the quality rows in the brief', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          {
            slug: SLUG,
            title: '제주',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
          },
        ],
        templates: POST_TEMPLATES,
      },
      templates: { templates: WITH_FIELDS },
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    // ① holds the post's material, and 분야 and 기억 사용 are not part of it.
    expect(await screen.findByLabelText('총평 별점')).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '분야' })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: '기억 사용' })).toBeNull()

    const brief = await openBrief(user)
    const tags = within(brief).getByLabelText('태그 개수')
    const quality = await within(brief).findByText('발행 글 점검')
    const field = within(brief).getByRole('group', { name: '분야' })
    const memory = within(brief).getByRole('checkbox', { name: '기억 사용' })
    const save = within(brief).getByRole('button', { name: '저장' })
    expect(following(tags, quality)).toBe(true)
    expect(following(quality, field)).toBe(true)
    expect(following(field, memory)).toBe(true)
    expect(following(memory, save)).toBe(true)
  })

  it('saves the brief’s run options together and keeps them across a reload', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const optionSaves: FakeOptionsSave[] = []
    const first = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, optionSaves, posts: [{ slug: SLUG, title: '제주' }] },
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    const brief = await openBrief(user)
    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    await user.clear(within(brief).getByLabelText('목표 글자 수'))
    await user.type(within(brief).getByLabelText('목표 글자 수'), '1500')
    await user.click(await m1())
    await user.click(within(brief).getByRole('button', { name: '카페' }))
    await user.click(within(brief).getByRole('checkbox', { name: '기억 사용' }))
    // Every change so far is the form's: nothing has been sent.
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(within(brief).getByRole('button', { name: '저장' }))
    await briefClosed()
    expect(optionSaves).toEqual([
      {
        slug: SLUG,
        targetLength: 1500,
        tagCount: 4,
        useMemory: true,
        qualityRules: ['title_saturation'],
        field: 'cafe',
      },
    ])

    const shows = async () => {
      const reopened = await openBrief(user)
      expect(within(reopened).getByLabelText('목표 글자 수')).toHaveValue(1500)
      expect(await m1()).toBeChecked()
      expect(within(reopened).getByRole('button', { name: '카페' })).toHaveAttribute(
        'aria-pressed',
        'true',
      )
      expect(within(reopened).getByRole('checkbox', { name: '기억 사용' })).toBeChecked()
    }
    await shows()

    first.unmount()
    renderAppAt(`/posts/${SLUG}`, { transport: first.transport })
    await shows()
    expect(optionSaves).toHaveLength(1)
  })

  it('offers no run options before the first save, and the create carries no 분야', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves },
      quality: { calls, accounts: { [SLUG]: M1_OVER_BAND } },
    })

    expect(await screen.findByLabelText('제목')).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '분야' })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: '기억 사용' })).toBeNull()

    const brief = await openBrief(user)
    expect(await within(brief).findByRole('combobox', { name: /작성 모델/ })).toBeInTheDocument()
    expect(within(brief).queryByRole('checkbox', { name: '목표 글자 수 사용' })).toBeNull()
    expect(within(brief).queryByLabelText('태그 개수')).toBeNull()
    expect(within(brief).queryByText('발행 글 점검')).toBeNull()
    expect(within(brief).queryByRole('group', { name: '분야' })).toBeNull()
    expect(within(brief).queryByRole('checkbox', { name: '기억 사용' })).toBeNull()
    expect(calls.filter((call) => call === 'GetAccountQuality')).toHaveLength(0)
    await user.keyboard('{Escape}')

    await user.type(screen.getByLabelText('제목'), '리뷰 글')
    await waitFor(() => expect(draftSaves).toHaveLength(1), { timeout: 4_000 })
    expect(draftSaves[0]).toMatchObject({ slug: '', field: undefined })
  })
})

// POST-81: ①'s brief reads the account aggregate and offers a tick on an over-band metric; the
// ticks autosave per post, where the next run's enqueue reads them.
describe('the brief quality rows', () => {
  const SLUG = '20260901-seongsu'
  const m1 = () => screen.findByRole('checkbox', { name: /^제목 도배율 42%/ })
  const reads = (calls: string[]) => calls.filter((call) => call === 'GetAccountQuality').length

  it('saves a tick and keeps it across a reload', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const optionSaves: FakeOptionsSave[] = []
    const first = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: {
        calls,
        optionSaves,
        posts: [{ slug: SLUG, title: '성수 카페', targetLength: 1800 }],
      },
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    // Read as soon as the dock renders, before anyone opens the brief.
    await screen.findByRole('tab', { name: '글 생성' })
    await waitFor(() => expect(reads(calls)).toBe(1))
    const brief = await openBrief(user)
    const box = await m1()
    expect(box).not.toBeChecked()
    await user.click(box)
    await user.click(within(brief).getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(optionSaves).toEqual([
        {
          slug: SLUG,
          targetLength: 1800,
          tagCount: 4,
          useMemory: false,
          qualityRules: ['title_saturation'],
          field: '',
        },
      ]),
    )
    // The start requests carry nothing new: the enqueue reads the saved ticks (T344).
    expect(calls).not.toContain('StartGeneration')

    first.unmount()
    renderAppAt(`/posts/${SLUG}`, { transport: first.transport })
    await openBrief(user)
    expect(await m1()).toBeChecked()
  })

  it('re-reads the aggregate when the target language changes', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, posts: [{ slug: SLUG, title: '성수 카페' }] },
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    const language = await briefField(user, '글 언어')
    await m1()
    const before = reads(calls)
    await user.click(language)
    await user.click(await screen.findByRole('option', { name: '영어' }))

    await waitFor(() => expect(calls).toContain('SavePostDraft'))
    await waitFor(() => expect(reads(calls)).toBeGreaterThan(before))
    // The server renders rule texts in the post's stored target language, and the read carries
    // only the slug, so a read after the language save is the rows reading the new language.
    expect(calls.lastIndexOf('GetAccountQuality')).toBeGreaterThan(calls.indexOf('SavePostDraft'))
  })

  it('disables the ticks while a job runs', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          {
            slug: SLUG,
            title: '성수 카페',
            activeJob: { id: 'job-1', kind: 'generate', status: 'running' },
          },
        ],
      },
      jobs: { jobs: [{ id: 'job-1', kind: 'generate', status: 'running' }] },
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    await openBrief(user)
    expect(await m1()).toBeDisabled()
  })
})
