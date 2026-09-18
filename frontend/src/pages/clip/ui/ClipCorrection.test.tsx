import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipSourceBatchSchema } from '@/shared/api'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeJobsOptions } from '@/test/jobs'

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
  initializeI18n('ko')
})

function fixture(): FakeClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: 'template',
    ratio: 'vertical',
    targetDurationMs: 19800,
    answers: [],
    disclosure: 'ad',
    cta: '',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: clipTimelineFixture(),
    result: {
      contentType: 'video/mp4',
      bytes: 5,
      durationMs: 19800,
      createdAt: '2026-09-10T00:00:00Z',
      viewUrl: 'https://private.test/old',
      downloadUrl: 'https://private.test/download',
    },
  }
}
async function mount(clips: FakeClipsOptions = {}, jobs: FakeJobsOptions = {}) {
  const view = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: { models: [] },
    jobs,
    clips: {
      templates: [
        {
          id: 'template',
          name: '여행',
          informationFields: [],
          cutGuidance: '',

          accent: '',
          preset: 'restaurant',
        },
      ],
      projects: [fixture()],
      ...clips,
    },
  })
  await screen.findByRole('heading', { name: '컷·자막 수정' })
  return view
}
async function goToStep(name: '클립 생성' | '클립 다듬기' | '클립 완성') {
  await userEvent.click(await screen.findByRole('tab', { name }))
}
const timeline = () => within(screen.getByLabelText('편집 타임라인'))
// A selection is what OPENS an item's sheet (CLIP-53), and ② arrives with none:
// every cut or caption control below is reached by selecting its bar first.
const selectCut = async (name = '컷 1') => userEvent.click(timeline().getByRole('button', { name }))
const selectText = async (name = 'caption a') =>
  userEvent.click(timeline().getByRole('button', { name }))
const setField = (label: string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
// No 저장 button: the draft autosaves on a pause as ①'s settings do (CLIP-39), so a
// test waits for the write that pause produces instead of pressing anything.
const AUTOSAVE = { timeout: 3000 }
async function select(ids = ['a', 'b']) {
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
  const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  const input = screen.getByLabelText('원본 영상 선택')
  await waitFor(() => expect(input).toBeEnabled())
  await userEvent.upload(input, files)
  return revoke
}

it('opens a matching result in refine with one action bar and an available download', async () => {
  await mount()
  expect(screen.getByRole('tab', { name: '클립 다듬기' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
    'href',
    'https://private.test/download',
  )
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeDisabled()
  // Nothing is selected, so no cut or caption control stands in the page.
  expect(screen.queryByLabelText('원본 시작 (초)')).not.toBeInTheDocument()
  await selectCut()
  expect(screen.getAllByLabelText('원본 시작 (초)')).toHaveLength(1)
  await goToStep('클립 생성')
  expect(screen.queryByRole('button', { name: '다시 렌더' })).not.toBeInTheDocument()
})

it('saves exact milliseconds and selected text with a new optimistic revision', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  await selectCut()
  setField('원본 시작 (초)', '0.123')
  setField('원본 끝 (초)', '12.345')
  await selectText()
  setField('자막 원문', '오늘 장면')
  // An autosave may finish while the owner switches from the cut to its text.
  // Every accepted write advances the optimistic revision; the last holds both edits.
  await waitFor(() => expect(writes.at(-1)?.plan.elements?.[0].text).toBe('오늘 장면'), AUTOSAVE)
  expect(writes.at(-1)!.plan.cuts[0]).toMatchObject({ startMs: 123, endMs: 12345 })
  expect(writes.map((write) => write.revision)).toEqual(writes.map((_, index) => index + 1))
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeInTheDocument()
})

// CLIP-53: one selection is ONE sheet. The page behind keeps the preview and the
// timeline, the item's own controls open over them, and the notice naming that
// item rides inside its sheet rather than anywhere in ②'s flow.
it('opens the selected cut or caption in its own sheet, with its notice, and closes to no selection', async () => {
  const p = fixture()
  p.notices = [
    { code: 'plan_cut_rate', cutId: 'cut-a', elementId: '', action: 'repair' },
    { code: 'plan_style', cutId: 'cut-a', elementId: 'caption', action: 'repair' },
    { code: 'plan_target_duration', cutId: '', elementId: '', action: 'shortfall' },
  ]
  const cutNotice = '이 장면은 원래 속도인 1배속으로 담았어요.'
  const captionNotice = '이 문구는 템플릿에서 정한 디자인으로 담았어요.'
  await mount({ projects: [p] })
  const page = () => within(screen.getByRole('region', { name: '컷·자막 수정' }))

  // Nothing is selected: no sheet, no item control and no targeted notice.
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('원본 시작 (초)')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('자막 원문')).not.toBeInTheDocument()
  expect(screen.queryByText(cutNotice)).not.toBeInTheDocument()
  expect(screen.queryByText(captionNotice)).not.toBeInTheDocument()
  // The one naming neither stands in the page.
  // T246 moves it into the preview's info control; today it also stands in the
  // section above the dock, which is why this counts rather than asserting one.
  expect(
    page().getAllByText('사용할 수 있는 장면만 담아 선택한 길이보다 짧게 완성됐어요.').length,
  ).toBeGreaterThan(0)

  await selectCut()
  const cutSheet = screen.getByRole('dialog', { name: /컷 1/ })
  expect(within(cutSheet).getByLabelText('원본 시작 (초)')).toBeInTheDocument()
  expect(within(cutSheet).getByRole('slider', { name: '원본 소리 (%)' })).toBeInTheDocument()
  expect(within(cutSheet).getByRole('button', { name: '아래로 이동' })).toBeInTheDocument()
  expect(within(cutSheet).getByRole('button', { name: '컷 삭제' })).toBeInTheDocument()
  expect(within(cutSheet).getByText(cutNotice)).toBeInTheDocument()
  expect(page().queryByText(cutNotice)).not.toBeInTheDocument()
  // The page behind kept both.
  expect(page().getByRole('region', { name: '편집 중인 영상' })).toBeInTheDocument()
  expect(page().getByLabelText('편집 타임라인')).toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 1' })).toHaveAttribute('aria-pressed', 'true')

  await userEvent.click(within(cutSheet).getByRole('button', { name: '닫기' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 1' })).toHaveAttribute('aria-pressed', 'false')

  await selectText()
  const textSheet = screen.getByRole('dialog', { name: '선택한 문구' })
  expect(within(textSheet).getByLabelText('자막 원문')).toBeInTheDocument()
  // The placement stage, over the output frame of the cut its interval starts in.
  expect(await within(textSheet).findByText('자막 배치')).toBeInTheDocument()
  expect(
    within(textSheet).getByRole('button', { name: /자막을 끌어서 옮기기/ }),
  ).toBeInTheDocument()
  expect(within(textSheet).getByText(captionNotice)).toBeInTheDocument()
  expect(page().queryByText(captionNotice)).not.toBeInTheDocument()
  expect(within(textSheet).getByRole('button', { name: '문구 삭제' })).toBeInTheDocument()

  await userEvent.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: 'caption a' })).toHaveAttribute(
    'aria-pressed',
    'false',
  )
})

it('keeps a selected text field mounted while its cut is reordered', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  await selectText()
  const field = screen.getByLabelText('자막 원문')
  setField('자막 원문', '선택한 문구')
  await userEvent.click(screen.getByRole('button', { name: '아래로 이동' }))
  expect(screen.getByLabelText('자막 원문')).toBe(field)
  expect(field).toHaveValue('선택한 문구')
  await waitFor(() => expect(writes).toHaveLength(1), AUTOSAVE)
  expect(writes[0].plan.cuts.map((c) => c.id)).toEqual(['cut-b', 'cut-a'])
})

it('preserves invalid authored intervals and disables rerender until repaired', async () => {
  const calls: string[] = []
  await mount({ calls })
  await selectText()
  setField('표시 끝 (초)', '12')
  expect(screen.getByLabelText('표시 끝 (초)')).toHaveValue(12)
  expect(screen.queryByRole('button', { name: '저장' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeDisabled()
  expect(screen.getByText(/이 문구의 내용·위치·시간/)).toBeInTheDocument()
  // The autosave does not take an invalid draft either, so nothing reaches the server.
  await new Promise((resolve) => setTimeout(resolve, 900))
  expect(calls).not.toContain('SaveClipEditPlan')
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  expect(screen.getByLabelText('표시 끝 (초)')).toHaveValue(3.88)
})

it('undoes a saved deletion, preserving output-relative fixed text', async () => {
  const p = fixture()
  p.editing!.plan.cuts.forEach((c) => {
    c.endMs = 20000
  })
  p.editing!.plan.durationMs = 39800
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ projects: [p], planWrites: writes })
  await selectCut()
  await userEvent.click(screen.getByRole('button', { name: '컷 삭제' }))
  expect(timeline().queryByRole('button', { name: 'caption a' })).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '정확한 고정 문구' })).toBeInTheDocument()
  await waitFor(() => expect(writes).toHaveLength(1), AUTOSAVE)
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  expect(timeline().getByRole('button', { name: 'caption a' })).toBeInTheDocument()
  await waitFor(() => expect(writes).toHaveLength(2), AUTOSAVE)
  expect(writes.map((w) => w.revision)).toEqual([1, 2])
})

it('edits fixed global text without requiring legacy campaign facts', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  await selectText('정확한 고정 문구')
  setField('자막 원문', '직접 쓴 문구 <그대로>')
  await waitFor(() => expect(writes).toHaveLength(1), AUTOSAVE)
  expect(writes[0].plan.elements!.find((t) => t.instanceId === 'global')).toMatchObject({
    text: '직접 쓴 문구 <그대로>',
    basis: 'whole',
    kind: 'fixed',
  })
})

it('edits rapid phrase timing in exact milliseconds on a rapid-paced clip', async () => {
  // The PACE is the project's (CLIP-139): ② edits the phrases of a clip that is
  // already rapid rather than switching one caption to it.
  const p = fixture()
  p.captionPace = 'rapid'
  p.editing!.plan.elements![0] = {
    ...p.editing!.plan.elements![0],
    pace: 'rapid',
    text: '오늘은 구로디지털단지에 와보았는데요',
    phrases: [
      { text: '오늘은', startMs: 120, endMs: 420 },
      { text: '구로디지털단지에', startMs: 420, endMs: 1120 },
      { text: '와보았는데요', startMs: 1120, endMs: 1820 },
    ],
  }
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ projects: [p], planWrites: writes })
  await selectText('오늘은')
  expect(screen.getByLabelText('구절 1')).toHaveValue('오늘은')
  // No per-caption pace or accent control survives in ②.
  expect(screen.queryByRole('combobox', { name: /^자막 흐름/ })).not.toBeInTheDocument()
  expect(screen.queryByLabelText('강조색')).not.toBeInTheDocument()
  const ends = screen.getAllByLabelText('컷 안에서 끝 (초)')
  fireEvent.change(ends[1], { target: { value: '0.919' } })
  await waitFor(() => expect(writes).toHaveLength(1), AUTOSAVE)
  expect(writes[0].plan.elements![0].phrases![1]).toMatchObject({
    text: '구로디지털단지에',
    startMs: 420,
    endMs: 919,
  })
})

it('keeps the local draft on conflict and reloads only after an explicit discard', async () => {
  await mount({ planSaveConflict: true })
  await selectText()
  setField('자막 원문', '내 수정')
  await screen.findByRole('button', { name: '내 편집을 최신 버전에 적용' }, AUTOSAVE)
  expect(screen.getByLabelText('자막 원문')).toHaveValue('내 수정')
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeInTheDocument()
})

it('requires matching sources for rerender while text edits and previous video remain usable', async () => {
  const calls: string[] = []
  await mount({ calls })
  await selectText()
  setField('자막 원문', '원본 없이 수정')
  await waitFor(() => expect(calls).toContain('SaveClipEditPlan'), AUTOSAVE)
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeDisabled()
  await select()
  await waitFor(() => expect(screen.getByRole('button', { name: '다시 렌더' })).toBeEnabled())
  expect(calls).not.toContain('StartClipGeneration')
})

it('uses localized selected-cut controls and shows a single audio control', async () => {
  await mount()
  await selectCut()
  await act(async () => initializeI18n('en'))
  expect(screen.getByRole('heading', { name: 'Edit cuts and captions' })).toBeInTheDocument()
  expect(screen.getByLabelText('Source start (seconds)')).toHaveValue(0)
  expect(screen.getAllByLabelText('Original audio (%)')).toHaveLength(1)
})

it('snaps range gestures to the output frame grid while precise typing retains milliseconds', async () => {
  await mount()
  await selectCut()
  fireEvent.change(screen.getByRole('slider', { name: '시작 손잡이 · 원본' }), {
    target: { value: '124' },
  })
  expect(screen.getByLabelText('원본 시작 (초)')).toHaveValue(0.133)
  setField('원본 시작 (초)', '0.124')
  expect(screen.getByLabelText('원본 시작 (초)')).toHaveValue(0.124)
  expect(screen.getByRole('button', { name: '현재 프레임을 시작으로' })).toBeDisabled()
})

it('adds observed footage through the page, then saves split/rate operations and their undo', async () => {
  const project = fixture()
  const source = project.editing!.sources[0]
  project.observations = {
    status: 'available',
    sources: [
      {
        source,
        segments: [
          {
            startMs: 12000,
            endMs: 15000,
            event: 'extra footage',
            action: '',
            motion: '',
            speech: '',
            subjects: [],
            quality: '',
            certainty: 'certain',
            usability: 'usable',
            focal: { x: 0.3, y: 0.6 },
          },
        ],
      },
    ],
  }
  const planWrites: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ projects: [project], planWrites })
  await userEvent.click(screen.getByRole('button', { name: /관찰 구간 1개 자세히 보기/ }))
  await userEvent.click(screen.getByRole('button', { name: '이 구간을 컷으로 추가' }))
  await waitFor(() => expect(planWrites).toHaveLength(1))
  const added = planWrites[0].plan.cuts[1]
  expect(added.id).toMatch(
    /^owner-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
  )
  expect(added).toMatchObject({
    startMs: 12000,
    endMs: 15000,
    playbackRatePermille: 1000,
    copies: [],
    chips: [],
    focal: { x: 0.3, y: 0.6 },
  })
  expect(screen.getByText(/2번 컷에 사용 · 1× · 원본 0:12–0:15/)).toBeVisible()
  await userEvent.click(screen.getByRole('combobox', { name: /재생 속도/ }))
  await userEvent.click(screen.getByRole('option', { name: '2×' }))
  await waitFor(() => expect(planWrites).toHaveLength(2))
  expect(planWrites[1].plan.cuts[1].playbackRatePermille).toBe(2000)
  fireEvent.change(screen.getByRole('spinbutton', { name: '나눌 원본 시간 (초)' }), {
    target: { value: '13.5' },
  })
  await userEvent.click(screen.getByRole('button', { name: '입력한 시간에서 나누기' }))
  await waitFor(() => expect(planWrites).toHaveLength(3))
  expect(planWrites[2].plan.cuts).toHaveLength(4)
  expect(planWrites[2].plan.cuts[2]).toMatchObject({
    startMs: 13500,
    endMs: 15000,
    playbackRatePermille: 2000,
    transitionMs: 0,
  })
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  await waitFor(() => expect(planWrites).toHaveLength(4))
  expect(planWrites[3].plan.cuts).toHaveLength(3)
  expect(planWrites.map((w) => w.revision)).toEqual([1, 2, 3, 4])
})

it('shares retained sound across both steps, stales the old render and offers credit-free rerender', async () => {
  const project = fixture()
  project.editing!.plan.sourceAudio = project.editing!.plan.cuts.map((c) => ({
    sourceId: c.sourceId,
    fingerprint: c.fingerprint,
    retainOriginalAudio: false,
  }))
  const batch = create(ClipSourceBatchSchema, {
    id: 'retained',
    projectId: 'clip',
    current: true,
    state: 'ready',
    expiresAt: '2099-01-01T00:00:00Z',
    sources: project.editing!.sources.map((s) => ({
      id: s.id,
      state: 'ready',
      availability: 'available',
      retentionExpiresAt: '2099-01-01T00:00:00Z',
      metadata: { ...s, contentType: 'video/mp4', bytes: 5n },
    })),
  })
  const soundWrites: NonNullable<FakeClipsOptions['soundWrites']> = [],
    calls: string[] = []
  await mount({ projects: [project], retainedBatches: [batch], soundWrites, calls })
  const toggle = await screen.findByRole('switch', { name: /source-a.mp4 원본 소리 유지/ })
  await waitFor(() => expect(toggle).toBeEnabled())
  expect(toggle).not.toBeChecked()
  await userEvent.click(toggle)
  await waitFor(() => expect(soundWrites).toHaveLength(1))
  await waitFor(() => expect(screen.getByRole('button', { name: '다시 렌더' })).toBeEnabled())
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeVisible()
  expect(calls).not.toContain('SaveClipEditPlan')
  await goToStep('클립 생성')
  expect(screen.getByRole('switch', { name: /source-a.mp4 원본 소리 유지/ })).toBeChecked()
  await userEvent.click(screen.getByRole('switch', { name: /source-a.mp4 원본 소리 유지/ }))
  await waitFor(() => expect(soundWrites).toHaveLength(2))
  expect(soundWrites.map((w) => w.expectedRevision)).toEqual([1, 2])
  expect(calls).not.toContain('StartClipGeneration')
  expect(calls).not.toContain('QuoteClipGeneration')
  await goToStep('클립 다듬기')
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  await waitFor(() => expect(soundWrites).toHaveLength(3))
  expect(soundWrites[2]).toMatchObject({ expectedRevision: 3, retainOriginalAudio: true })
})

// CLIP-39: ② autosaves as ① does, so 저장 is gone and a dirty draft is not a reason to
// refuse 다시 렌더 — the render flushes the queue itself and runs against what the server
// took, never against a revision the owner has since typed past.
it('renders a dirty draft by flushing it first, and offers no 저장 anywhere', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  const calls: string[] = []
  await mount({ planWrites: writes, calls })
  await select()
  await waitFor(() => expect(screen.getByRole('button', { name: '다시 렌더' })).toBeEnabled())
  await selectText()
  setField('자막 원문', '렌더 직전 수정')
  // Dirty, and no way to save it by hand.
  expect(screen.queryByRole('button', { name: '저장' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeEnabled()
  await userEvent.click(screen.getByRole('button', { name: '다시 렌더' }))
  await waitFor(() => expect(calls).toContain('StartClipRender'), AUTOSAVE)
  // The edit reached the server BEFORE the render started.
  expect(writes.at(-1)?.plan.elements?.[0].text).toBe('렌더 직전 수정')
  expect(calls.indexOf('SaveClipEditPlan')).toBeLessThan(calls.indexOf('StartClipRender'))
})
