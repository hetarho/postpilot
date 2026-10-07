import { localSamplesForControls } from '@/test/clip-local-samples'
import { afterEach, expect, it, vi } from 'vitest'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { hashKey } from '@tanstack/react-query'
import { initializeI18n } from '@/app/providers/i18n'
import { Code } from '@connectrpc/connect'
import { Stage } from '@/shared/api'
import { clipRegionRows } from '@/entities/clip-plan'
import { myPlanQueryKey } from '@/entities/plan'
import { discardClipDraftQueues } from '@/features/edit-clip-project'
import { discardClipRegionQueues } from '@/features/edit-clip-regions'
import { discardClipStorylineQueues } from '@/features/edit-clip-storyline'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { renderAppAt } from '@/test/app'
import { connectAppError } from '@/test/app-error'
import { clipObservationsFixture } from '@/test/clip-observations'
import { clipTimelineFixture } from '@/test/clip-editing'
import { projectFakeRegions, type FakeClipProject, type FakeClipsOptions } from '@/test/clips'
import type { FakeJobsOptions } from '@/test/jobs'
import { chooseOption } from '@/test/listbox'
import type { FakeProvidersOptions } from '@/test/providers'

vi.mock('@/features/upload-clip-sources/model/manifest', async (original) => ({
  ...(await original<object>()),
  readSourceManifest: vi.fn(),
}))
vi.mock('@/shared/lib/upload', async (original) => ({
  ...(await original<object>()),
  putBlobWithProgress: vi.fn(),
}))
afterEach(() => {
  vi.restoreAllMocks()
  discardClipDraftQueues()
  discardClipRegionQueues()
  discardClipStorylineQueues()
  initializeI18n('ko')
})

const models: FakeProvidersOptions = {
  models: [
    {
      providerId: 'p',
      modelId: 'o',
      label: 'Video observer',
      vision: true,
      videoInput: true,
      inlineStaticVideo: true,
      stages: [Stage.OBSERVE],
    },
    { providerId: 'p', modelId: 'w', label: 'Writer', stages: [Stage.WRITE] },
  ],
  selections: [
    { stage: Stage.OBSERVE, providerId: 'p', modelId: 'o' },
    { stage: Stage.WRITE, providerId: 'p', modelId: 'w' },
  ],
}

function storylined(extra: Partial<FakeClipProject> = {}): FakeClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: 'template',
    ratio: 'vertical',
    targetDurationMs: 19800,
    disclosure: 'ad',
    observations: clipObservationsFixture(),
    storyline: {
      paragraphs: [
        { text: '음식을 가까이 보여줘요.', observationIds: ['a/0'] },
        { text: '테이블로 마무리해요.', observationIds: [] },
      ],
      editedByHand: false,
      addedSourceIds: [],
      takenOutObservationIds: ['a/1'],
    },
    ...extra,
  }
}

async function mount(
  project: FakeClipProject,
  clips: FakeClipsOptions = {},
  jobs: FakeJobsOptions = {},
  wait: 'space' | 'tab' | 'none' = 'space',
) {
  const view = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: models,
    jobs,
    clips: {
      templates: [{ id: 'template', name: '여행', compositionBody: '<clip version="1"/>' }],
      projects: [project],
      eligibility: [{ providerId: 'p', modelId: 'o', status: 'eligible' }],
      ...clips,
    },
  })
  if (wait === 'space') await screen.findByRole('button', { name: '스토리라인' })
  if (wait === 'tab') await screen.findByRole('tab', { name: '수정' })
  return view
}

const heading = () => screen.getByRole('button', { name: '스토리라인' })
const paragraph = (n: number) => within(screen.getByRole('listitem', { name: `${n}번째 문단` }))
const AUTOSAVE = { timeout: 3000 }

type Kind = 'intro' | 'outro'
const slot = (kind: Kind, n: number, text = '', extra = {}) => ({
  id: `project-${kind}-${n}`,
  instruction: '',
  text,
  instructionEdited: false,
  ownerFixed: false,
  bound: false,
  ...extra,
})
type Regions = NonNullable<FakeClipProject['regions']>
function regions(intro: Partial<Regions['intro']> = {}, outro: Partial<Regions['outro']> = {}) {
  return {
    revision: 1,
    intro: { enabled: true, slots: [slot('intro', 1), slot('intro', 2)], ...intro },
    outro: {
      enabled: false,
      slots: [slot('outro', 1, '다시 만나요', { ownerFixed: true }), slot('outro', 2)],
      ...outro,
    },
  }
}
/** A project with its slots and nothing else yet: no template, no storyline, no plan. */
function bare(extra: Partial<FakeClipProject> = {}): FakeClipProject {
  return {
    id: 'clip',
    title: '성수',
    videoTemplateId: '',
    ratio: 'vertical',
    targetDurationMs: 19800,
    disclosure: 'ad',
    introPreset: 'a',
    outroPreset: 'b',
    regions: regions(),
    ...extra,
  }
}

function regionSafeEditing() {
  const editing = clipTimelineFixture()
  const first = editing.plan.elements!.find((text) => text.instanceId === 'caption-a')!
  first.startMs = first.resolvedStartMs = 2600
  first.endMs = first.resolvedEndMs = 6360
  return editing
}
const block = (name: '인트로' | '아웃트로') => within(screen.getByRole('region', { name }))
const line = (name: '인트로' | '아웃트로', n: number) =>
  within(block(name).getAllByRole('listitem')[n - 1])
/** What the region writes asked for, folded per slot into its latest fields. */
function sent(writes: NonNullable<FakeClipsOptions['regionWrites']>, kind: Kind) {
  const out: Record<string, { instruction?: string; text?: string }> = {}
  for (const write of writes)
    for (const edit of write[kind]?.slots ?? [])
      out[edit.id] = {
        ...out[edit.id],
        ...(edit.instruction !== undefined ? { instruction: edit.instruction } : {}),
        ...(edit.text !== undefined ? { text: edit.text } : {}),
      }
  return out
}

// CLIP-36, CLIP-177: a storyline without a plan is already ②, where its space is open — the
// storyline is what the owner works on until something is built from it.
it('opens ② on a storyline with no plan, its space open with paragraphs and scenes', async () => {
  await mount(storylined())
  expect(screen.getByRole('tab', { name: '수정' })).toHaveAttribute('aria-selected', 'true')
  expect(heading()).toHaveAttribute('aria-expanded', 'true')
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
  expect(paragraph(1).getByText('장면 1')).toBeVisible()
  // No footage in the session: the scene shows what was observed in it instead of a frame.
  expect(paragraph(1).getByText('접시에 담긴 음식을 가까이 촬영')).toBeVisible()
  const out = within(screen.getByRole('region', { name: '빠진 장면' }))
  expect(out.getByText('장면 2')).toBeVisible()
})

// CLIP-179: with a plan the space starts closed — the plan is what ② is for then.
it('closes the space when ② shows a plan', async () => {
  await mount(
    storylined({ editPlanRevision: 1, renderedPlanRevision: 0, editing: clipTimelineFixture() }),
  )
  expect(screen.queryByRole('listitem', { name: '1번째 문단' })).not.toBeInTheDocument()
  await userEvent.click(heading())
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
})

// CLIP-178, CLIP-39: an edit saves itself — the text, a scene moved, a scene taken out and put
// back — through the project update, with no save button.
it('saves the owner’s edits by themselves', async () => {
  const storylineEdits: NonNullable<FakeClipsOptions['storylineEdits']> = []
  await mount(storylined(), { storylineEdits })
  const user = userEvent.setup()
  await user.click(paragraph(2).getByRole('button', { name: '2번째 문단 고치기' }))
  const field = paragraph(2).getByRole('textbox', { name: '2번째 문단' })
  await user.clear(field)
  await user.type(field, '테이블을 비추며 끝내요.')
  await waitFor(() => expect(storylineEdits.at(-1)?.[1]?.text).toBe('테이블을 비추며 끝내요.'), {
    timeout: 3000,
  })
  await user.click(paragraph(2).getByRole('button', { name: '완료' }))

  await user.click(paragraph(1).getByRole('button', { name: '장면 1 옮기기' }))
  await user.click(await screen.findByRole('menuitemradio', { name: '2번째 문단' }))
  await waitFor(() =>
    expect(storylineEdits.at(-1)?.map((p) => p.observationIds)).toEqual([[], ['a/0']]),
  )

  await user.click(paragraph(2).getByRole('button', { name: '장면 1 옮기기' }))
  await user.click(await screen.findByRole('menuitemradio', { name: '빼기' }))
  await waitFor(() => expect(storylineEdits.at(-1)?.map((p) => p.observationIds)).toEqual([[], []]))
  const out = within(screen.getByRole('region', { name: '빠진 장면' }))
  expect(out.getByText('장면 1')).toBeVisible()

  await user.click(out.getByRole('button', { name: '장면 2 넣기' }))
  await user.click(await screen.findByRole('menuitem', { name: '1번째 문단' }))
  await waitFor(() =>
    expect(storylineEdits.at(-1)?.map((p) => p.observationIds)).toEqual([['a/1'], []]),
  )
})

// CLIP-39: the storyline saves through the clip's autosave machine — a transient failure is sent
// again, and no failure hands the owner's words back to the server's.
it('retries a storyline save the network dropped, keeping the typed words', async () => {
  const storylineEdits: NonNullable<FakeClipsOptions['storylineEdits']> = []
  let offline = true
  await mount(storylined(), {
    storylineEdits,
    projectSaveGate: async () => {
      if (offline) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    },
  })
  const user = userEvent.setup()
  await user.click(paragraph(2).getByRole('button', { name: '2번째 문단 고치기' }))
  const field = paragraph(2).getByRole('textbox', { name: '2번째 문단' })
  await user.clear(field)
  await user.type(field, '테이블을 비추며 끝내요.')
  expect(await screen.findByRole('alert', {}, AUTOSAVE)).toBeVisible()
  expect(field).toHaveValue('테이블을 비추며 끝내요.')
  expect(storylineEdits).toHaveLength(0)
  offline = false
  await waitFor(() => expect(storylineEdits.at(-1)?.[1]?.text).toBe('테이블을 비추며 끝내요.'), {
    timeout: 5000,
  })
  await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  expect(field).toHaveValue('테이블을 비추며 끝내요.')
})

it('keeps the typed words and says why when the server refuses the storyline', async () => {
  const storylineEdits: NonNullable<FakeClipsOptions['storylineEdits']> = []
  await mount(storylined(), { storylineEdits, storylineEditFails: true })
  const user = userEvent.setup()
  await user.click(paragraph(2).getByRole('button', { name: '2번째 문단 고치기' }))
  const field = paragraph(2).getByRole('textbox', { name: '2번째 문단' })
  await user.clear(field)
  await user.type(field, '테이블을 비추며 끝내요.')
  expect(await screen.findByRole('alert', {}, AUTOSAVE)).toBeVisible()
  expect(storylineEdits.at(-1)?.[1]?.text).toBe('테이블을 비추며 끝내요.')
  // Refused, not retried, and the words stay the owner's rather than the server's.
  await new Promise((resolve) => setTimeout(resolve, 1500))
  expect(storylineEdits).toHaveLength(1)
  expect(field).toHaveValue('테이블을 비추며 끝내요.')
})

// CLIP-178: footage added after the storyline was made is said, with what to do about it.
it('says when footage was added after the storyline was made', async () => {
  await mount(storylined({ storyline: { ...storylined().storyline!, addedSourceIds: ['c'] } }))
  expect(
    screen.getByText('이 스토리라인을 만든 뒤 영상이 추가됐어요. 다시 만들면 새 영상도 들어가요.'),
  ).toBeVisible()
})

// CLIP-160: a finalized clip and a running job leave the storyline to be read, not edited.
it.each([
  [
    'finalized',
    storylined({
      editPlanRevision: 1,
      renderedPlanRevision: 1,
      editing: clipTimelineFixture(),
      finalized: { at: '2026-09-12T00:00:00Z', planRevision: 1, resultId: 'result' },
      regions: regions(),
      result: {
        id: 'result',
        contentType: 'video/mp4',
        bytes: 5,
        durationMs: 19800,
        createdAt: '2026-09-10T00:00:00Z',
        viewUrl: 'https://private.test/view',
      },
    }),
  ],
])('reads the storyline without controls when %s', async (_, project) => {
  await mount(project, {}, {}, 'tab')
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
  await screen.findByRole('button', { name: '스토리라인' })
  if (heading().getAttribute('aria-expanded') !== 'true') await userEvent.click(heading())
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
  expect(paragraph(1).queryByRole('button', { name: '1번째 문단 고치기' })).not.toBeInTheDocument()
  expect(paragraph(1).queryByRole('button', { name: '장면 1 옮기기' })).not.toBeInTheDocument()
  // A finalized clip carries none of the space's actions (CLIP-160).
  expect(screen.queryByRole('button', { name: '다시 만들기' })).not.toBeInTheDocument()
  expect(screen.queryByLabelText('스토리라인 수정 요청')).not.toBeInTheDocument()
  // Its slots are read where they stand, with nothing to switch or type (CLIP-179).
  expect(block('인트로').queryByRole('switch')).not.toBeInTheDocument()
  expect(line('인트로', 1).getByLabelText('1번째 줄 문구')).toHaveAttribute('readonly')
})

// A job that ② owns keeps ② on screen, and the storyline is read-only under it.
it('reads the storyline without controls while a job runs', async () => {
  const job = {
    id: 'running',
    kind: 'revise_clip',
    status: 'running',
    stage: 'narrate',
    clipProjectId: 'clip',
  }
  await mount(
    storylined({
      editPlanRevision: 1,
      renderedPlanRevision: 1,
      editing: clipTimelineFixture(),
      latestJob: job,
      regions: regions(),
    }),
    {},
    { jobs: [job] },
    'tab',
  )
  await userEvent.click(await screen.findByRole('button', { name: '스토리라인' }))
  expect(paragraph(1).getByText('음식을 가까이 보여줘요.')).toBeVisible()
  expect(paragraph(1).queryByRole('button', { name: '1번째 문단 고치기' })).not.toBeInTheDocument()
  expect(block('인트로').queryByRole('switch')).not.toBeInTheDocument()
  expect(line('인트로', 1).getByLabelText('1번째 줄 들어갈 내용')).toHaveAttribute('readonly')
  // Every action is held while the job runs (CLIP-181).
  expect(screen.getByRole('button', { name: '다시 만들기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '이 스토리로 다시 만들기' })).toBeDisabled()
  expect(screen.getByLabelText('스토리라인 수정 요청')).toBeDisabled()
})

// CLIP-38: the focused run view and the status line name the storyline call's stage.
it('names the storyline stage while the storyline call runs', async () => {
  const job = {
    id: 'running',
    kind: 'storyline_clip',
    status: 'running',
    stage: 'storyline',
    clipProjectId: 'clip',
  }
  await mount(storylined({ latestJob: job }), {}, { jobs: [job] }, 'none')
  expect(await screen.findAllByText('스토리라인 작성 중')).not.toHaveLength(0)
})

// CLIP-177, CLIP-181: a storyline call settles as its job ends and carries no accounting to wait
// on, so its end is what moves the header's balance — once.
it.each(['storyline_clip', 'revise_storyline_clip'])(
  'stales the balance once when a %s job ends',
  async (kind) => {
    const job = {
      id: 'storyline',
      kind,
      status: 'running',
      stage: 'storyline',
      clipProjectId: 'clip',
    }
    const view = await mount(storylined({ latestJob: job }), {}, { jobs: [job] }, 'none')
    await waitFor(() =>
      expect(view.queryClient.getQueryData(myPlanQueryKey(view.transport))).toBeTruthy(),
    )
    const invalidate = vi.spyOn(view.queryClient, 'invalidateQueries')
    const plan = hashKey(myPlanQueryKey(view.transport))
    const balanceStales = () =>
      invalidate.mock.calls.filter(
        (call) => call[0]?.queryKey && hashKey(call[0].queryKey) === plan,
      ).length
    job.status = 'done'
    for (let read = 0; read < 3; read++)
      await act(() => view.queryClient.refetchQueries({ type: 'active' }))
    await waitFor(() => expect(balanceStales()).toBe(1))
    await act(() => view.queryClient.refetchQueries({ type: 'active' }))
    expect(balanceStales()).toBe(1)
  },
)

// CLIP-179, CLIP-186: the slots stand in ② before any template, storyline or plan, and an edit
// saves itself as a region edit naming the revision it was made over.
it('writes the slots on ② before any template, storyline or plan', async () => {
  const regionWrites: NonNullable<FakeClipsOptions['regionWrites']> = []
  await mount(bare(), { regionWrites, templates: [] }, {}, 'tab')
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
  expect(heading()).toHaveAttribute('aria-expanded', 'true')
  expect(block('인트로').getByText('A 크기만')).toBeVisible()
  // The renderer's own numbered drawing of the chosen preset, as ① shows it (CLIP-165).
  await waitFor(() =>
    expect(
      screen.getByRole('region', { name: '인트로' }).querySelector('svg [data-preset="intro-a"]'),
    ).toHaveTextContent('슬롯 1'),
  )
  expect(block('아웃트로').getByText('사용 안 함')).toBeVisible()
  expect(block('아웃트로').queryByLabelText('1번째 줄 문구')).not.toBeInTheDocument()
  // No body to edit and no storyline action to build one from: ① starts that.
  expect(screen.queryByRole('listitem', { name: '1번째 문단' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '다시 만들기' })).not.toBeInTheDocument()
  expect(screen.getByText(/아직 다듬을 편집안이 없어요/)).toBeVisible()
  expect(block('인트로').getByText('아직 영상에 보일 문구가 없어요.')).toBeVisible()

  const user = userEvent.setup()
  await user.type(line('인트로', 1).getByLabelText('1번째 줄 문구'), '성수 로컬')
  await user.type(line('인트로', 2).getByLabelText('2번째 줄 들어갈 내용'), '영업 시간')
  await waitFor(
    () =>
      expect(sent(regionWrites, 'intro')).toEqual({
        'project-intro-1': { text: '성수 로컬' },
        'project-intro-2': { instruction: '영업 시간' },
      }),
    AUTOSAVE,
  )
  expect(regionWrites[0].expectedRegionRevision).toBe(1)
  expect(line('인트로', 1).getByText('직접 입력')).toBeVisible()
  // An instruction alone leaves the slot waiting for words (CLIP-186).
  expect(line('인트로', 2).getByText('생성 대기')).toBeVisible()
  expect(block('인트로').queryByText('아직 영상에 보일 문구가 없어요.')).not.toBeInTheDocument()
})

// CLIP-111: ① offers 사용 안 함 beside the presets. Off keeps the slots; choosing the preset
// the region already has turns it back on explicitly, since no preset change would.
it('turns a region off and on from ①, keeping its slots', async () => {
  const regionWrites: NonNullable<FakeClipsOptions['regionWrites']> = []
  await mount(bare(), { regionWrites }, {}, 'tab')
  await userEvent.setup().click(await screen.findByRole('button', { name: '디자인과 자막 스타일' }))
  const intro = await screen.findByRole('radiogroup', { name: '인트로 디자인' })
  const outro = screen.getByRole('radiogroup', { name: '아웃트로 디자인' })
  expect(within(intro).getByRole('radio', { name: 'A 크기만' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  expect(within(outro).getByRole('radio', { name: '사용 안 함' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await userEvent.click(within(intro).getByRole('radio', { name: '사용 안 함' }))
  await waitFor(() => expect(regionWrites.at(-1)?.intro?.enabled).toBe(false), AUTOSAVE)
  await userEvent.click(within(outro).getByRole('radio', { name: 'B 가로선 구분' }))
  await waitFor(() => expect(regionWrites.at(-1)?.outro?.enabled).toBe(true), AUTOSAVE)
  expect(
    regionWrites.flatMap((write) => [write.intro, write.outro]).flatMap((r) => r?.slots ?? []),
  ).toEqual([])
  expect(within(intro).getByRole('radio', { name: '사용 안 함' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
  expect(block('인트로').getByText('사용 안 함')).toBeVisible()
  expect(line('아웃트로', 1).getByLabelText('1번째 줄 문구')).toHaveValue('다시 만나요')
})

// CLIP-168, T450: what a template seeds is the server's to decide — the browser shows its
// answer, with the owner's own words kept over the template's.
it('shows the server’s seed when ① chooses a template, the owner’s words kept', async () => {
  const user = userEvent.setup()
  await mount(
    bare({
      regions: regions({
        enabled: false,
        slots: [slot('intro', 1, '내가 쓴 첫 줄', { ownerFixed: true }), slot('intro', 2)],
      }),
    }),
    { templateRegions: { template: { intro: ['템플릿 첫 줄', '템플릿 둘째 줄'] } } },
    {},
    'tab',
  )
  await userEvent.setup().click(await screen.findByRole('button', { name: '디자인과 자막 스타일' }))
  const intro = await screen.findByRole('radiogroup', { name: '인트로 디자인' })
  expect(within(intro).getByRole('radio', { name: '사용 안 함' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await chooseOption(user, screen.getByRole('combobox', { name: /^영상 템플릿/ }), '여행')
  await waitFor(
    () =>
      expect(within(intro).getByRole('radio', { name: 'A 크기만' })).toHaveAttribute(
        'aria-checked',
        'true',
      ),
    AUTOSAVE,
  )
  await user.click(screen.getByRole('tab', { name: '수정' }))
  expect(line('인트로', 1).getByLabelText('1번째 줄 문구')).toHaveValue('내가 쓴 첫 줄')
  expect(line('인트로', 1).getByText('직접 입력')).toBeVisible()
  expect(line('인트로', 2).getByLabelText('2번째 줄 문구')).toHaveValue('템플릿 둘째 줄')
  expect(line('인트로', 2).getByText('생성됨')).toBeVisible()
})

// CLIP-186, CDS-73: clearing a slot's words is the owner's choice of an empty line, told apart
// from a slot still waiting for words.
it('keeps a deliberate blank apart from a slot awaiting words', async () => {
  const regionWrites: NonNullable<FakeClipsOptions['regionWrites']> = []
  await mount(
    bare({ regions: regions({ slots: [slot('intro', 1, '생성된 첫 줄'), slot('intro', 2)] }) }),
    { regionWrites },
    {},
    'tab',
  )
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
  expect(line('인트로', 1).getByText('생성됨')).toBeVisible()
  await userEvent.clear(line('인트로', 1).getByLabelText('1번째 줄 문구'))
  expect(line('인트로', 1).getByText('비워 둠')).toBeVisible()
  expect(line('인트로', 2).getByText('생성 대기')).toBeVisible()
  await waitFor(
    () => expect(sent(regionWrites, 'intro')).toEqual({ 'project-intro-1': { text: '' } }),
    AUTOSAVE,
  )
  expect(line('인트로', 1).getByText('비워 둠')).toBeVisible()
})

// CLIP-147, CLIP-189: words past the preset's slots stay as unused content with their notice;
// moving them into a slot keeps the words they replace, and a preset with room restores them.
it('keeps unused words, moves them into a slot and restores them with room', async () => {
  const regionWrites: NonNullable<FakeClipsOptions['regionWrites']> = []
  const user = userEvent.setup()
  await mount(
    bare({
      regions: regions({
        slots: [
          slot('intro', 1, '첫 줄', { ownerFixed: true }),
          slot('intro', 2, '둘째 줄', { ownerFixed: true }),
          slot('intro', 3, '셋째 줄', { ownerFixed: true }),
        ],
      }),
      notices: [
        { code: 'region_line_surplus', cutId: '', elementId: 'project-intro-3', action: 'removal' },
      ],
    }),
    { regionWrites },
    {},
    'tab',
  )
  await user.click(screen.getByRole('tab', { name: '수정' }))
  const unused = () => within(screen.getByRole('region', { name: '쓰지 않는 문구' }))
  expect(unused().getByLabelText('쓰지 않는 문구 1')).toHaveValue('셋째 줄')
  expect(unused().getByText(/담을 수 있는 줄 수를 넘겨서/)).toBeVisible()
  await user.click(unused().getByRole('button', { name: '쓰지 않는 문구 1 옮기기' }))
  await user.click(await screen.findByRole('menuitem', { name: '1번째 줄로 옮기기' }))
  expect(line('인트로', 1).getByLabelText('1번째 줄 문구')).toHaveValue('셋째 줄')
  expect(unused().getByLabelText('쓰지 않는 문구 1')).toHaveValue('첫 줄')
  await waitFor(
    () =>
      expect(sent(regionWrites, 'intro')).toEqual({
        'project-intro-1': { text: '셋째 줄' },
        'project-intro-3': { text: '첫 줄' },
      }),
    AUTOSAVE,
  )

  // Off and on again: every word is where it was (CLIP-189).
  await user.click(block('인트로').getByRole('switch', { name: '인트로 사용' }))
  expect(block('인트로').queryByLabelText('1번째 줄 문구')).not.toBeInTheDocument()
  await user.click(block('인트로').getByRole('switch', { name: '인트로 사용' }))
  expect(line('인트로', 1).getByLabelText('1번째 줄 문구')).toHaveValue('셋째 줄')

  // A preset with three slots draws the third again.
  await user.click(screen.getByRole('tab', { name: '생성' }))
  await userEvent.setup().click(await screen.findByRole('button', { name: '디자인과 자막 스타일' }))
  const intro = await screen.findByRole('radiogroup', { name: '인트로 디자인' })
  await user.click(within(intro).getByRole('radio', { name: '매거진 커버' }))
  await user.click(screen.getByRole('tab', { name: '수정' }))
  await waitFor(
    () => expect(line('인트로', 3).getByLabelText('3번째 줄 문구')).toHaveValue('첫 줄'),
    AUTOSAVE,
  )
  expect(screen.queryByRole('region', { name: '쓰지 않는 문구' })).not.toBeInTheDocument()
})

// CLIP-189, CDS-64: words the slot cannot draw are refused in their field and kept there, and
// they are not sent; the rest of the edit is.
it('refuses words a slot cannot draw in its field and keeps them', async () => {
  const regionWrites: NonNullable<FakeClipsOptions['regionWrites']> = []
  await mount(bare(), { regionWrites }, {}, 'tab')
  await userEvent.click(screen.getByRole('tab', { name: '수정' }))
  const field = line('인트로', 1).getByLabelText('1번째 줄 문구')
  // Paperlogy does not draw 갂 (CDS-17).
  fireEvent.change(field, { target: { value: '갂' } })
  fireEvent.change(line('인트로', 1).getByLabelText('1번째 줄 들어갈 내용'), {
    target: { value: '가게 이름' },
  })
  expect(field).toHaveAttribute('aria-invalid', 'true')
  expect(field).toHaveAccessibleDescription('이 줄의 글꼴로 쓸 수 없는 글자가 있어요.')
  await waitFor(
    () =>
      expect(sent(regionWrites, 'intro')).toEqual({
        'project-intro-1': { instruction: '가게 이름' },
      }),
    AUTOSAVE,
  )
  expect(field).toHaveValue('갂')
  fireEvent.change(field, { target: { value: '가게' } })
  expect(field).not.toHaveAttribute('aria-invalid')
  await waitFor(
    () => expect(sent(regionWrites, 'intro')['project-intro-1']?.text).toBe('가게'),
    AUTOSAVE,
  )
})

const RENDER = /^(렌더하기|다시 렌더)$/
async function selectSources(ids = ['a', 'b']) {
  const files = ids.map((id) => new File(['clip'], `source-${id}.mp4`, { type: 'video/mp4' }))
  vi.mocked(readSourceManifest).mockResolvedValue(
    files.map((f, i) => ({
      filename: f.name,
      contentType: f.type,
      bytes: f.size,
      width: 1920,
      height: 1080,
      durationMs: 40000,
      fingerprint: ids[i]!.repeat(64),
    })),
  )
  vi.mocked(putBlobWithProgress).mockResolvedValue()
  vi.spyOn(URL, 'createObjectURL').mockImplementation((blob) => `blob:${(blob as File).name}`)
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  await userEvent.click(screen.getByRole('button', { name: '원본 소스' }))
  await userEvent.click(screen.getByRole('tab', { name: '원본 영상' }))
  const input = screen.getByLabelText('원본 영상 선택')
  await waitFor(() => expect(input).toBeEnabled())
  await userEvent.upload(input, files)
  await userEvent.keyboard('{Escape}')
}

// CLIP-39, CLIP-188: a render flushes every queue in order. A slot save held in flight lands
// first and moves the plan; the correction typed meanwhile is carried onto that plan rather than
// refused as a conflict or sent with the old region words; the render names what both left.
it('lands a delayed slot save before the correction and the render it precedes', async () => {
  const calls: string[] = []
  const planWrites: NonNullable<FakeClipsOptions['planWrites']> = []
  const renderStarts: unknown[] = []
  let release!: () => void
  const held = new Promise<void>((resolve) => {
    release = resolve
  })
  const project = storylined({
    videoTemplateId: '',
    introPreset: 'a',
    outroPreset: 'b',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: regionSafeEditing(),
    regions: regions({
      slots: [slot('intro', 1, '성수 로컬', { ownerFixed: true }), slot('intro', 2)],
    }),
    result: {
      contentType: 'video/mp4',
      bytes: 5,
      durationMs: 19800,
      createdAt: '2026-09-10T00:00:00Z',
      viewUrl: 'https://private.test/old',
    },
  })
  // The plan already draws the intro, as the server projected it (T451).
  projectFakeRegions(project)
  project.renderedPlanRevision = project.editPlanRevision
  await mount(project, { calls, planWrites, renderStarts, regionSaveGate: () => held }, {}, 'tab')
  await screen.findByRole('region', { name: '컷·자막 수정' })
  await selectSources()
  await waitFor(() => expect(screen.getByRole('button', { name: RENDER })).toBeEnabled())

  await userEvent.click(heading())
  await userEvent.click(block('아웃트로').getByRole('switch', { name: '아웃트로 사용' }))
  await userEvent.click(
    within(screen.getByLabelText('편집 타임라인')).getByRole('button', { name: 'caption a' }),
  )
  await userEvent.click(screen.getByRole('button', { name: '상세 편집' }))
  fireEvent.change(screen.getByLabelText('자막 원문'), { target: { value: '렌더 직전 수정' } })
  await userEvent.click(screen.getByRole('button', { name: RENDER }))
  await userEvent.click(await screen.findByRole('button', { name: '서버에서 렌더' }))

  // The slot save is out and held; nothing that depends on it has gone.
  await waitFor(() => expect(calls).toContain('UpdateClipProject'))
  expect(calls).not.toContain('SaveClipEditPlan')
  expect(calls).not.toContain('StartClipRender')
  release()
  await waitFor(() => expect(calls).toContain('StartClipRender'), AUTOSAVE)
  expect(calls.indexOf('UpdateClipProject')).toBeLessThan(calls.indexOf('SaveClipEditPlan'))
  expect(calls.lastIndexOf('SaveClipEditPlan')).toBeLessThan(calls.indexOf('StartClipRender'))
  const saved = planWrites.at(-1)!.plan
  expect(saved.elements?.find((t) => t.instanceId === 'caption-a')?.text).toBe('렌더 직전 수정')
  expect(clipRegionRows(saved, 'outro')).toEqual(['다시 만나요', ''])
  expect(clipRegionRows(saved, 'intro')).toEqual(['성수 로컬', ''])
  expect(renderStarts.at(-1)).toMatchObject({ expectedRevision: planWrites.at(-1)!.revision + 1 })
  expect(screen.queryByText(/저장된 수정본이 변경되었어요/)).not.toBeInTheDocument()
})

// CLIP-188: the storyline block and the edit plan hold one set of region words. A slot typed in
// the block is the plan's row — the correction saves it and the draft preview has it at once —
// and a row typed in correction is the slot's words.
it('shares one set of region words between the storyline block and the correction', async () => {
  const planWrites: NonNullable<FakeClipsOptions['planWrites']> = []
  const regionWrites: NonNullable<FakeClipsOptions['regionWrites']> = []
  const project = storylined({
    videoTemplateId: '',
    introPreset: 'a',
    outroPreset: 'b',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: regionSafeEditing(),
    regions: regions({
      slots: [slot('intro', 1, '성수 로컬', { ownerFixed: true }), slot('intro', 2)],
    }),
  })
  projectFakeRegions(project)
  await mount(project, { planWrites, regionWrites }, {}, 'tab')
  await screen.findByRole('region', { name: '컷·자막 수정' })
  await userEvent.click(heading())
  // The space reads 인트로 → body → 아웃트로 (CLIP-179).
  const [intro, body, outro] = [
    screen.getByRole('region', { name: '인트로' }),
    screen.getByRole('listitem', { name: '1번째 문단' }),
    screen.getByRole('region', { name: '아웃트로' }),
  ]
  expect(intro.compareDocumentPosition(body) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(body.compareDocumentPosition(outro) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  fireEvent.change(line('인트로', 2).getByLabelText('2번째 줄 문구'), {
    target: { value: '저녁 영업' },
  })
  expect(line('인트로', 2).getByText('직접 입력')).toBeVisible()
  await waitFor(
    () =>
      expect(planWrites.at(-1) && clipRegionRows(planWrites.at(-1)!.plan, 'intro')).toEqual([
        '성수 로컬',
        '저녁 영업',
      ]),
    AUTOSAVE,
  )
  // The words went as the plan's row, not as a second copy in a region edit.
  expect(sent(regionWrites, 'intro')).toEqual({})

  await userEvent.click(
    within(screen.getByLabelText('편집 타임라인')).getByRole('button', { name: /성수 로컬/ }),
  )
  await userEvent.click(screen.getByRole('button', { name: '상세 편집' }))
  expect(screen.getByLabelText('문구 2행')).toHaveValue('저녁 영업')
  fireEvent.change(screen.getByLabelText('문구 1행'), { target: { value: '성수 로컬 가이드' } })
  expect(line('인트로', 1).getByLabelText('1번째 줄 문구')).toHaveValue('성수 로컬 가이드')
  await waitFor(
    () =>
      expect(clipRegionRows(planWrites.at(-1)!.plan, 'intro')).toEqual([
        '성수 로컬 가이드',
        '저녁 영업',
      ]),
    AUTOSAVE,
  )
  expect(line('인트로', 1).getByLabelText('1번째 줄 문구')).toHaveValue('성수 로컬 가이드')
})

/** A clip whose plan draws its intro and whose current render matches it, with an id a
 *  finalization can name: the state every committing action below starts from. */
function rendered(): FakeClipProject {
  const project = storylined({
    videoTemplateId: '',
    introPreset: 'a',
    outroPreset: 'b',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: regionSafeEditing(),
    regions: regions({
      slots: [slot('intro', 1, '성수 로컬', { ownerFixed: true }), slot('intro', 2)],
    }),
    canFinalize: true,
    result: {
      id: 'result-1',
      contentType: 'video/mp4',
      bytes: 5,
      durationMs: 19800,
      createdAt: '2026-09-10T00:00:00Z',
      viewUrl: 'https://private.test/old',
    },
  })
  projectFakeRegions(project)
  project.renderedPlanRevision = project.editPlanRevision
  return project
}
async function editCaption(text: string) {
  await userEvent.click(
    within(screen.getByLabelText('편집 타임라인')).getByRole('button', { name: 'caption a' }),
  )
  await userEvent.click(screen.getByRole('button', { name: '상세 편집' }))
  fireEvent.change(screen.getByLabelText('자막 원문'), { target: { value: text } })
}
async function renderOnServer() {
  await userEvent.click(screen.getByRole('button', { name: RENDER }))
  await userEvent.click(await screen.findByRole('button', { name: '서버에서 렌더' }))
}

// CLIP-39, CLIP-188: a render flushes every queue first; a slot save that fails stops it, and
// the edits that were not saved stay where the owner left them.
it('launches no render past a slot save that fails, keeping both edits', async () => {
  const calls: string[] = []
  await mount(
    rendered(),
    { calls, projectSaveError: connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable) },
    {},
    'tab',
  )
  await screen.findByRole('region', { name: '컷·자막 수정' })
  await selectSources()
  await userEvent.click(heading())
  await userEvent.click(block('아웃트로').getByRole('switch', { name: '아웃트로 사용' }))
  await editCaption('렌더 직전 수정')
  await renderOnServer()
  await waitFor(() => expect(calls).toContain('UpdateClipProject'), AUTOSAVE)
  await new Promise((resolve) => setTimeout(resolve, 300))
  expect(calls).not.toContain('StartClipRender')
  expect(block('아웃트로').getByRole('switch', { name: '아웃트로 사용' })).toBeChecked()
  expect(screen.getByLabelText('자막 원문')).toHaveValue('렌더 직전 수정')
})

// A correction the server refuses as a conflict stops the render too, and says so.
it('launches no render past a correction save the server refuses as a conflict', async () => {
  const calls: string[] = []
  await mount(rendered(), { calls, planSaveConflict: true }, {}, 'tab')
  await screen.findByRole('region', { name: '컷·자막 수정' })
  await selectSources()
  await editCaption('충돌하는 수정')
  await renderOnServer()
  await waitFor(() => expect(calls).toContain('SaveClipEditPlan'), AUTOSAVE)
  expect(await screen.findByText(/저장된 수정본이 변경되었어요/)).toBeInTheDocument()
  expect(calls).not.toContain('StartClipRender')
  expect(screen.getByLabelText('자막 원문')).toHaveValue('충돌하는 수정')
})

// CDS-64, CLIP-188: words a slot cannot draw stay in the field and hold every committing
// action — the render cannot even be asked for.
it('holds the render while a slot holds words it cannot draw', async () => {
  const calls: string[] = []
  await mount(rendered(), { calls }, {}, 'tab')
  await screen.findByRole('region', { name: '컷·자막 수정' })
  await selectSources()
  await editCaption('다시 렌더할 수정')
  await waitFor(() => expect(screen.getByRole('button', { name: RENDER })).toBeEnabled(), AUTOSAVE)
  await userEvent.click(heading())
  const field = line('인트로', 1).getByLabelText('1번째 줄 문구')
  fireEvent.change(field, { target: { value: '갂' } })
  expect(field).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByRole('button', { name: RENDER })).toBeDisabled()
  expect(field).toHaveValue('갂')
  expect(calls).not.toContain('StartClipRender')
})

// CLIP-152, CLIP-188: once a slot edit moves the plan past the render, that render is named by
// its own revision and cannot be confirmed as the clip.
it('never confirms an earlier render for a plan a slot edit moved', async () => {
  await mount(rendered(), {}, {}, 'tab')
  await screen.findByRole('region', { name: '컷·자막 수정' })
  expect(screen.getByRole('button', { name: '확정하기' })).toBeEnabled()
  await userEvent.click(heading())
  fireEvent.change(line('인트로', 2).getByLabelText('2번째 줄 문구'), {
    target: { value: '저녁 영업' },
  })
  await waitFor(
    () => expect(screen.getByRole('button', { name: '확정하기' })).toBeDisabled(),
    AUTOSAVE,
  )
  expect(screen.getByText('렌더하기로 현재 편집안을 출력한 뒤 확정해 주세요.')).toBeInTheDocument()
})

vi.mock('@/entities/clip-preview/ui/useLocalSamples', () => localSamplesForControls)
