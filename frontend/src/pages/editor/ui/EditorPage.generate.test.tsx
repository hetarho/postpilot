// ① 글 생성: starting a run, the re-observation picker, the photos and the writing brief's run
// options and quality rows.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
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
import type { FakeGenerationStart } from '@/test/jobs'
import type { FakeDraftSave, FakeOptionsSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

/** A run over a post with no photo or video asks first (POST-108); go on without one. */
async function confirmNoPhotos(user: ReturnType<typeof userEvent.setup>) {
  const dialog = await screen.findByRole('dialog', { name: '사진 없이 만들까요?' })
  await user.click(within(dialog).getByRole('button', { name: '사진 없이 만들기' }))
}

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
    // never spelled out as prose in the dock (POST-46).
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

  // GEN-68, POST-44: 스토리라인 먼저 starts a storyline job from ①, the job names its stage, and the
  // draft that comes back holding a storyline opens ②.
  it('starts a storyline from ① and opens ② once the storyline lands', async () => {
    const slug = '20260820-memo'
    const calls: string[] = []
    const storylineStarts: FakeGenerationStart[] = []
    const user = userEvent.setup()
    const draft = { slug, memo: '노포에 갔다' }
    const withStoryline = {
      ...draft,
      storyline: {
        paragraphs: [{ text: '노포에 간 이유를 보여줍니다.' }, { text: '마무리합니다.' }],
      },
    }
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      // The read on mount, the one the running job refreshes, and the one its completion refreshes.
      posts: { posts: [draft], getSequence: [draft, draft, withStoryline] },
      jobs: {
        storylineStarts,
        sequence: [
          {
            id: 'job-started',
            kind: 'storyline',
            status: 'running',
            stage: 'storyline',
            progressTotal: 1,
          },
          {
            id: 'job-started',
            kind: 'storyline',
            status: 'done',
            stage: 'storyline',
            progressDone: 1,
            progressTotal: 1,
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    const storyline = await screen.findByRole('button', { name: '스토리라인 먼저' })
    await waitFor(() => expect(storyline).toBeEnabled())
    await user.click(storyline)
    await confirmNoPhotos(user)
    await waitFor(() => expect(calls).toContain('StartStoryline'))
    expect(storylineStarts[0]).toMatchObject({
      postSlug: slug,
      writeModel: { providerId: 'openrouter', modelId: 'writer' },
    })
    expect(calls).not.toContain('StartGeneration')
    await waitFor(
      () =>
        expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
          'aria-selected',
          'true',
        ),
      { timeout: 4_000 },
    )
  })

  // GEN-25: a post with 말투 없음 needs no voice to run, and nothing warns about the absence.
  it('lets a zero-photo post with 말투 없음 generate', async () => {
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      posts: { posts: [{ slug: '20260820-memo', memo: '사진 없는 메모', voice: null }] },
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
    await waitFor(() => expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeEnabled())
    // Neither voice refusal — deleted or not yet made — has anything to say.
    expect(screen.queryByText(/말투예요/)).not.toBeInTheDocument()
    expect(screen.getByLabelText('글 작업').previousElementSibling).toHaveClass('h-6')
    expect(screen.getByLabelText('글 작업').previousElementSibling).not.toHaveClass('mt-auto')

    const brief = await openBrief(user)
    expect(within(brief).getByText('사진이 없어 관찰 모델은 필요하지 않아요.')).toBeInTheDocument()
  })

  it('points to a required experience answer before starting either writing path', async () => {
    const slug = '20260820-memo'
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: {
        posts: [{ slug, memo: '동네 가게 방문', template: { id: 'visit', name: '방문 기록' } }],
        templates: [{ id: 'visit', name: '방문 기록' }],
      },
      templates: {
        templates: [
          {
            id: 'visit',
            name: '방문 기록',
            body: '<ask label="방문 계기" required="true">직접 가게 된 이유</ask>',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    const answer = await screen.findByRole('textbox', { name: /방문 계기/ })
    const generate = screen.getByRole('button', { name: '바로 글 쓰기' })
    const storyline = screen.getByRole('button', { name: '스토리라인 먼저' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    expect(screen.getByText('필수 입력란을 확인해 주세요: 방문 계기')).toBeInTheDocument()
    expect(answer).toHaveFocus()
    expect(calls).not.toContain('StartGeneration')
    await user.click(storyline)
    expect(calls).not.toContain('StartStoryline')

    await user.type(answer, '퇴근길에 간판을 보고 처음 들어갔다.')
    await user.click(storyline)
    await confirmNoPhotos(user)
    await waitFor(() => expect(calls).toContain('StartStoryline'))
  })

  it('requires a previously excluded answer to be restored and saved before writing', async () => {
    const slug = '20260820-memo'
    const calls: string[] = []
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: {
        draftSaves,
        posts: [
          {
            slug,
            memo: '동네 가게 방문',
            template: { id: 'visit', name: '방문 기록' },
            templateAnswers: [{ label: '방문 계기', text: '간판을 보고 들어갔다', enabled: false }],
          },
        ],
        templates: [{ id: 'visit', name: '방문 기록' }],
      },
      templates: {
        templates: [
          {
            id: 'visit',
            name: '방문 기록',
            body: '<ask label="방문 계기" required="true">직접 가게 된 이유</ask>',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    const answer = await screen.findByRole('textbox', { name: /방문 계기/ })
    expect(answer).toBeEnabled()
    const generate = screen.getByRole('button', { name: '바로 글 쓰기' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    expect(calls).not.toContain('StartGeneration')
    expect(answer).toHaveFocus()

    await user.click(screen.getByRole('button', { name: '기존 답변 사용하기' }))
    await user.click(generate)
    await confirmNoPhotos(user)
    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(draftSaves.at(-1)?.templateAnswers).toContainEqual({
      label: '방문 계기',
      text: '간판을 보고 들어갔다',
      enabled: true,
    })
    expect(calls.indexOf('SavePostDraft')).toBeLessThan(calls.indexOf('StartGeneration'))
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

    const generate = await screen.findByRole('button', { name: '바로 글 쓰기' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.type(screen.getByLabelText('메모'), ' + 최신 내용')
    await user.click(generate)
    await confirmNoPhotos(user)

    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(calls.filter((call) => call === 'SavePostDraft' || call === 'StartGeneration')).toEqual([
      'SavePostDraft',
      'StartGeneration',
    ])
  })

  // GEN-8: a post that has already been observed decides what to re-observe BEFORE the
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

    const generate = await screen.findByRole('button', { name: '바로 글 쓰기' })
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
    await user.click(screen.getByRole('button', { name: '바로 글 쓰기' }))
    await user.click(await screen.findByRole('checkbox', { name: 'IMG_2.jpg 다시 관찰' }))
    await user.click(screen.getByRole('button', { name: '이대로 시작' }))
    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(starts).toHaveLength(1)
    expect(starts[0].reobserveFiles).toEqual(['IMG_2.jpg'])
    expect(calls.indexOf('SavePostDraft')).toBeGreaterThanOrEqual(0)
    expect(calls.indexOf('SavePostDraft')).toBeLessThan(calls.indexOf('StartGeneration'))
  })

  it('flushes the newest material before opening the common writing test without starting paid work', async () => {
    const calls: string[] = []
    const draftMaterials: Array<{ slug: string; title: string; memo: string }> = []
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-memo' }], draftMaterials },
    })
    await user.type(await screen.findByLabelText('메모'), '방금 입력한 경험')
    const brief = await openBrief(user)
    await user.click(within(brief).getByRole('link', { name: '글 설정 비교 테스트' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/tests'))
    expect(router.state.location.search).toMatchObject({ sourcePost: '20260820-memo' })
    expect(draftMaterials.at(-1)?.memo).toBe('방금 입력한 경험')
    expect(calls).toContain('SavePostDraft')
    expect(calls).not.toContain('StartWriteExperiment')
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
    const tags = within(brief).getByLabelText('최대 태그 수')
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
    expect(within(brief).queryByLabelText('최대 태그 수')).toBeNull()
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
