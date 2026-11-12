import { clipGuideTo } from '@/test/clip-preparation'
import { localSamplesForControls } from '@/test/clip-local-samples'
import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipSourceBatchSchema } from '@/shared/api'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { renderAppAt } from '@/test/app'
import { clipNarrationFixture, clipTimelineFixture } from '@/test/clip-editing'
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
    disclosure: 'ad',
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
          compositionBody: '<clip version="1"/>',
        },
      ],
      projects: [fixture()],
      ...clips,
    },
  })
  await screen.findByRole('region', { name: '컷·자막 수정' })
  return view
}

it('selects a caption in the primary preview, preserves its clock, edits placement and undoes without speech calls', async () => {
  const calls: string[] = [],
    planWrites: NonNullable<FakeClipsOptions['planWrites']> = []
  const project = fixture()
  project.editing!.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'a'.repeat(64),
    volumePermille: 700,
    segments: [
      {
        id: 'spoken-1',
        text: '읽는 대본',
        inputHash: 'b'.repeat(64),
        textRevision: 1,
        startMs: 0,
        endMs: 5000,
        speech: {
          assetId: 'asset',
          voiceId: 'voice',
          bindingDigest: 'a'.repeat(64),
          inputHash: 'b'.repeat(64),
          settingsHash: 'c'.repeat(64),
          audioHash: 'd'.repeat(64),
          profileId: 'profile',
          profileRevision: 1,
          samples: 44100,
          sampleRate: 44100,
          channels: 2,
          timing: [],
        },
      },
    ],
  }
  const speech = structuredClone(project.editing!.plan.narration)
  await mount({ projects: [project], calls, planWrites })
  fireEvent.change(screen.getByRole('slider', { name: '완성 영상 기준 시간' }), {
    target: { value: '2000' },
  })
  const caption = await screen.findByRole('button', { name: '미리보기 자막 선택: caption a' })
  await userEvent.click(caption)
  expect(screen.getByRole('slider', { name: '완성 영상 기준 시간' })).toHaveValue('2000')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  const handle = screen.getByRole('button', { name: /자막을 끌어서 옮기기/ })
  handle.focus()
  await userEvent.keyboard('{ArrowRight}')
  await waitFor(() => expect(planWrites).toHaveLength(1), { timeout: 3000 })
  expect(planWrites[0].plan.elements?.[0].ownerPosition).toEqual({ x: 248, y: 1500 })
  expect(planWrites[0].plan.narration).toEqual(speech)
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  await waitFor(() => expect(planWrites).toHaveLength(2), { timeout: 3000 })
  expect(planWrites[1].plan.elements?.[0].ownerPosition).toBeUndefined()
  expect(planWrites[1].plan.narration).toEqual(speech)
  expect(calls.some((call) => /Start|Quote|Synthesize/.test(call))).toBe(false)
})

it('explicitly refreshes derived wording while preserving owner words, timing and speech', async () => {
  const calls: string[] = [],
    planWrites: NonNullable<FakeClipsOptions['planWrites']> = []
  const project = fixture()
  project.editing = clipNarrationFixture()
  const plan = project.editing.plan
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'a'.repeat(64),
    volumePermille: 1000,
    segments: [
      {
        id: 's1',
        text: '새 대본 문구',
        inputHash: 'b'.repeat(64),
        textRevision: 2,
        startMs: 0,
        endMs: 11000,
      },
      {
        id: 's2',
        text: '고칠 다른 대본',
        inputHash: 'c'.repeat(64),
        textRevision: 2,
        startMs: 11000,
        endMs: 19000,
      },
    ],
  }
  plan.elements![0].derivedCaption = {
    segmentId: 's1',
    textRevision: 1,
    textEdited: false,
    timingEdited: true,
  }
  plan.elements![1].derivedCaption = {
    segmentId: 's2',
    textRevision: 1,
    textEdited: true,
    timingEdited: false,
  }
  const protectedCaption = structuredClone(plan.elements![1])
  await mount({ projects: [project], calls, planWrites })
  await userEvent.click(screen.getByRole('button', { name: '대본에서 자막 업데이트' }))
  await waitFor(() => expect(planWrites).toHaveLength(1), { timeout: 3000 })
  expect(planWrites[0].plan.elements![0]).toMatchObject({
    text: '새 대본 문구',
    startMs: plan.elements![0].startMs,
    endMs: plan.elements![0].endMs,
    derivedCaption: { textRevision: 2, textEdited: false, timingEdited: true },
  })
  expect(planWrites[0].plan.elements![1]).toMatchObject(protectedCaption)
  expect(planWrites[0].plan.narration).toEqual(plan.narration)
  expect(calls.some((call) => /Start|Quote|Synthesize/.test(call))).toBe(false)
})
async function goToStep(name: '영상 생성' | '영상 다듬기' | '영상 완성') {
  await userEvent.click(await screen.findByRole('tab', { name }))
}
const timeline = () => within(screen.getByRole('region', { name: '장면 블록' }))
// ②'s one render trigger: 렌더하기, or 다시 렌더 once a render of the current plan exists (CLIP-40).
// ② renders nothing (CLIP-149): 확정하기 is the committing action an invalid draft holds
// and a dirty one flushes before it runs (CLIP-39, CLIP-75).
const RENDER = '확정하기'
// Each block opens its own sheet from its own control (CLIP-55). Selection alone stays on the
// preview and the list; the dedicated test below pins that flow.
const selectCut = async (name = '컷 1') => {
  await userEvent.click(timeline().getByRole('button', { name: `${name} 편집` }))
}
// A caption opens its sheet from its own control in the block it starts in (CLIP-17, CLIP-143).
const selectText = async (name = 'caption a') => {
  const field = timeline().getByDisplayValue(name).closest<HTMLElement>('[data-scene-text]')!
  await userEvent.click(within(field).getByRole('button', { name: /편집$/ }))
}
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
  await userEvent.click(screen.getByRole('button', { name: '원본 소스' }))
  await userEvent.click(screen.getByRole('tab', { name: '원본 영상' }))
  const input = within(screen.getByRole('dialog', { name: '원본 소스' })).getByLabelText(
    '원본 영상 선택',
  )
  await waitFor(() => expect(input).toBeEnabled())
  await userEvent.upload(input, files)
  await userEvent.keyboard('{Escape}')
  return revoke
}

it('selects on a phone without opening details or suspending the preview, and keeps selection on close', async () => {
  const view = await mount()
  await userEvent.click(screen.getByRole('button', { name: '흐름 보기' }))
  const preview = view.container.querySelector('[data-flow-simulation]')
  await userEvent.click(timeline().getByRole('button', { name: '컷 1' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 1 편집' })).toBeInTheDocument()
  expect(view.container.querySelector('[data-flow-simulation]')).toBe(preview)
  await userEvent.click(timeline().getByRole('button', { name: '컷 1 편집' }))
  const details = screen.getByRole('dialog', { name: /컷 1/ })
  expect(view.container.querySelector('[data-flow-simulation]')).toBe(preview)
  await userEvent.click(within(details).getByRole('button', { name: '닫기' }))
  expect(timeline().getByRole('button', { name: '컷 1' })).toHaveAttribute('aria-pressed', 'true')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('edits a caption in place beside the desktop preview and keeps it focused during seek', async () => {
  const original = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    ...original(query),
    matches: /min-width/.test(query) || original(query).matches,
  }))
  await mount()
  // The properties no longer stand beside the preview: the selected block opens them (T764).
  expect(screen.queryByRole('complementary', { name: '선택 항목 속성' })).not.toBeInTheDocument()
  const field = timeline().getByDisplayValue('caption a')
  await userEvent.click(field)
  expect(field).toHaveAttribute('data-selected', 'true')
  const seek = screen.getByRole('slider', { name: '완성 영상 기준 시간' })
  fireEvent.change(seek, { target: { value: '5000' } })
  expect(timeline().getByDisplayValue('caption a')).toBe(field)
  expect(document.activeElement).toBe(field)
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('stands a caption-free draft as scene blocks and reaches its first caption and explicit dubbing setup', async () => {
  const p = fixture()
  p.editing!.plan.elements = []
  await mount({ projects: [p] })
  expect(timeline().getByRole('button', { name: '컷 1' })).toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 2' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '이 지점에 자막 추가' })).toBeEnabled()
  await userEvent.click(screen.getByRole('button', { name: '더빙 설정' }))
  const setup = within(screen.getByRole('dialog', { name: '더빙 대본' }))
  expect(setup.getByRole('link', { name: '내 목소리 보기' })).toHaveAttribute(
    'href',
    '/spoken-voices',
  )
})

it('selects speech by its stable identity without generating audio or changing the caption selection', async () => {
  const p = fixture(),
    calls: string[] = []
  p.editing!.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      {
        id: 'spoken-1',
        text: '읽을 별도 대본',
        textRevision: 1,
        inputHash: 'input',
        startMs: 3500,
        endMs: 5000,
      },
    ],
  }
  await mount({ projects: [p], calls })
  const speech = timeline().getByLabelText('더빙 1 · 음성 필요')
  await userEvent.click(speech)
  expect(speech).toHaveAttribute('data-selected', 'true')
  expect(timeline().getByDisplayValue('caption a')).toHaveAttribute('data-selected', 'false')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await userEvent.click(timeline().getByRole('button', { name: '더빙 1 편집' }))
  expect(
    within(screen.getByRole('dialog', { name: '더빙 1' })).getByText('읽을 별도 대본'),
  ).toBeVisible()
  expect(calls.filter((call) => /Start|Quote|Synth/.test(call))).toEqual([])
})

// CLIP-197, THEME-45: the AI request is the step bar's field, so no entry stands before it, and
// the script still opens through its named entry without unmounting the preview or the words.
it('keeps the AI request in the step bar and the script behind its named entry', async () => {
  await mount()
  const field = screen.getByLabelText('수정 요청을 입력하세요')
  expect(field).toBeVisible()
  expect(screen.queryByRole('button', { name: 'AI로 수정' })).not.toBeInTheDocument()
  const preview = screen.getByRole('region', { name: '편집 중인 영상' })
  await userEvent.type(field, '내 요청을 유지해 주세요')
  await userEvent.click(screen.getByRole('button', { name: '더빙 대본' }))
  await userEvent.keyboard('{Escape}')
  expect(screen.getByLabelText('수정 요청을 입력하세요')).toHaveValue('내 요청을 유지해 주세요')
  expect(screen.getByRole('region', { name: '편집 중인 영상' })).toBe(preview)
})

it('opens a matching result in refine with one action bar and an available download', async () => {
  await mount()
  expect(screen.getByRole('tab', { name: '영상 다듬기' })).toHaveAttribute('aria-selected', 'true')
  // ② carries no download: it stands under the delivered video in ③ (CLIP-149).
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /^(렌더하기|다시 렌더)$/ })).not.toBeInTheDocument()
  // Nothing is selected, so no cut or caption control stands in the page.
  expect(screen.queryByLabelText('원본 시작 (초)')).not.toBeInTheDocument()
  await selectCut()
  expect(screen.getAllByLabelText('원본 시작 (초)')).toHaveLength(1)
  await goToStep('영상 생성')
  expect(screen.queryByRole('button', { name: RENDER })).not.toBeInTheDocument()
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
  // ② carries no download: it stands under the delivered video in ③ (CLIP-149).
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
})

// F72, CLIP-39: ② has no leave-confirmation dialog; an edit still waiting for its autosave is
// sent as the owner leaves.
it('leaves ② without a dialog and saves the edit waiting for its autosave', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  const view = await mount({ planWrites: writes })
  await selectText()
  setField('자막 원문', '떠나기 전 장면')
  await act(() => view.router.navigate({ to: '/naver-clip' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/naver-clip'))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await waitFor(() => expect(writes.at(-1)?.plan.elements?.[0].text).toBe('떠나기 전 장면'))
})

// CLIP-53, CLIP-174: the selected item's sheet owns the visible source frame. The background
// preview releases its media while the sheet is open, then returns to the chosen view.
it('opens an item sheet over the flow view, and says the flow view is still frames', async () => {
  const view = await mount()
  await userEvent.click(screen.getByRole('button', { name: '흐름 보기' }))
  expect(view.container.querySelector('[data-flow-simulation]')).toBeInTheDocument()
  await selectCut()
  expect(screen.getByRole('dialog', { name: /컷 1/ })).toBeInTheDocument()
  expect(view.container.querySelector('[data-flow-simulation]')).toBeInTheDocument()
  await userEvent.keyboard('{Escape}')
  expect(view.container.querySelector('[data-flow-simulation]')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '이 미리보기에 대해' }))
  expect(
    await screen.findByText('멈춘 장면으로 흐름만 보여줘요. 실제 렌더와 다를 수 있어요.'),
  ).toBeInTheDocument()
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
  const captionNotice = '이 문구는 이 클립에서 고른 디자인으로 담았어요.'
  await mount({ projects: [p] })
  const page = () => within(screen.getByRole('region', { name: '컷·자막 수정' }))

  // Nothing is selected: no sheet, no item control and no targeted notice.
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('원본 시작 (초)')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('자막 원문')).not.toBeInTheDocument()
  expect(screen.queryByText(cutNotice)).not.toBeInTheDocument()
  expect(screen.queryByText(captionNotice)).not.toBeInTheDocument()
  // The one naming neither is reached from the info control and stands nowhere in
  // the flow (CLIP-148). The previous render's own comparison keeps a copy behind
  // its closed details (CLIP-152), so what is asserted is that none of it SHOWS.
  for (const node of screen.queryAllByText(
    '사용할 수 있는 장면만 담아 선택한 길이보다 짧게 완성됐어요.',
  ))
    expect(node).not.toBeVisible()

  await selectCut()
  const cutSheet = screen.getByRole('dialog', { name: /컷 1/ })
  expect(within(cutSheet).getByLabelText('원본 시작 (초)')).toBeInTheDocument()
  expect(within(cutSheet).getByRole('slider', { name: '원본 소리 (%)' })).toBeInTheDocument()
  // Reordering is the block's own control now, not the sheet's (THEME-32).
  expect(within(cutSheet).queryByRole('button', { name: '아래로 이동' })).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 1 아래로' })).toBeInTheDocument()
  expect(within(cutSheet).getByRole('button', { name: '컷 삭제' })).toBeInTheDocument()
  expect(within(cutSheet).getByText(cutNotice)).toBeInTheDocument()
  expect(page().queryByText(cutNotice)).not.toBeInTheDocument()
  // The page behind kept both.
  expect(page().getByRole('region', { name: '편집 중인 영상' })).toBeInTheDocument()
  expect(page().getByRole('region', { name: '장면 블록' })).toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 1' })).toHaveAttribute('aria-pressed', 'true')

  await userEvent.click(within(cutSheet).getByRole('button', { name: '닫기' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '컷 1' })).toHaveAttribute('aria-pressed', 'true')

  await selectText()
  const textSheet = screen.getByRole('dialog', { name: '선택한 문구' })
  expect(within(textSheet).getByLabelText('자막 원문')).toBeInTheDocument()
  // The caption's own sheet places it over a frame from the cut it starts in (CLIP-143).
  expect(within(textSheet).getByText('자막 배치')).toBeInTheDocument()
  expect(
    within(textSheet).getByText('미리보기에서 자막을 끌거나 방향키로 위치를 조절하세요.'),
  ).toBeVisible()
  expect(within(textSheet).getByText(captionNotice)).toBeInTheDocument()
  expect(within(textSheet).getByRole('button', { name: '문구 삭제' })).toBeInTheDocument()
  await userEvent.keyboard('{Escape}')
  expect(await screen.findByRole('button', { name: /자막을 끌어서 옮기기/ })).toBeVisible()
  expect(timeline().getByDisplayValue('caption a')).toHaveAttribute('data-selected', 'true')
})

// CLIP-53, CLIP-148, CLIP-149: the preview's own row. One scrubber stating its
// second, one info control holding everything ② has to say about the whole clip,
// and the download of the render this plan already has.
it('carries its scrubber, its info control and its download on the preview itself', async () => {
  const p = fixture()
  p.notices = [{ code: 'plan_target_duration', cutId: '', elementId: '', action: 'shortfall' }]
  const untargeted = '사용할 수 있는 장면만 담아 선택한 길이보다 짧게 완성됐어요.'
  const parity = /브라우저 재생 시간에는 오차가 있을 수 있어요/
  await mount({ projects: [p] })

  // One row: the control carries its own name and its seconds are its value, so
  // no label line is drawn for it.
  const scrubber = screen.getByRole('slider', { name: '완성 영상 기준 시간' })
  expect(scrubber).toHaveAttribute('aria-valuetext', '0 / 19.8 s')
  expect(document.querySelector(`label[for="${scrubber.id}"]`)).toBeNull()

  // None of what the info control holds stands as permanent text in the flow. The
  // copy inside the previous render's closed comparison is not text ② states.
  expect(screen.queryByText(parity)).not.toBeInTheDocument()
  for (const node of screen.queryAllByText(untargeted)) expect(node).not.toBeVisible()

  // The info control stands ON the frame, at its top-right, beside the player's own controls
  // (owner decision 2026-09-19).
  const about = screen.getByRole('button', { name: '이 미리보기에 대해' })
  const canvas = document.querySelector('[data-clip-preview-canvas]')!
  expect(canvas.parentElement).toContainElement(about)
  expect(canvas).not.toContainElement(about)
  const corner = within(canvas.parentElement!.lastElementChild as HTMLElement)
  // The flow view's control stands beside the info control (CLIP-174).
  expect(corner.getAllByRole('button').map((b) => b.getAttribute('aria-label'))).toEqual([
    '미리보기 소리 듣기',
    '미리보기 새로고침',
    '흐름 보기',
    '이 미리보기에 대해',
  ])
  await userEvent.click(about)
  const panel = within(screen.getByRole('dialog', { name: '이 미리보기에 대해' }))
  expect(panel.getByText(parity)).toBeInTheDocument()
  expect(panel.getByText(untargeted)).toBeInTheDocument()

  // ② carries no download: it stands under the delivered video in ③ (CLIP-149).
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
})

// A plan awaiting its first render is ②'s NORMAL first state (CLIP-56): the
// download is simply absent, and nothing reads as missing or refused.
it('offers no download while the plan has no render, and says nothing is wrong', async () => {
  const p = fixture()
  p.result = undefined
  p.renderedPlanRevision = 0
  await mount({ projects: [p] })
  expect(screen.getByRole('tab', { name: '영상 다듬기' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.queryByRole('link', { name: /다운로드/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '이 미리보기에 대해' })).toBeInTheDocument()
  expect(screen.getByRole('slider', { name: '완성 영상 기준 시간' })).toBeInTheDocument()
})

it('keeps a selected text field mounted while its cut is reordered', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  const field = timeline().getByDisplayValue('caption a')
  await userEvent.click(field)
  fireEvent.change(field, { target: { value: '선택한 문구' } })
  await userEvent.click(timeline().getByRole('button', { name: '컷 1 아래로' }))
  expect(timeline().getByDisplayValue('선택한 문구')).toBe(field)
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
  expect(screen.getByRole('button', { name: RENDER })).toBeDisabled()
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
  expect(timeline().queryByDisplayValue('caption a')).not.toBeInTheDocument()
  expect(timeline().getByText('정확한 고정 문구')).toBeInTheDocument()
  await waitFor(() => expect(writes).toHaveLength(1), AUTOSAVE)
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  expect(timeline().getByDisplayValue('caption a')).toBeInTheDocument()
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
  await selectText('오늘은 구로디지털단지에 와보았는데요')
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
  // ② carries no download: it stands under the delivered video in ③ (CLIP-149).
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
})

it('requires matching sources for rerender while text edits and previous video remain usable', async () => {
  const calls: string[] = []
  await mount({ calls })
  await selectText()
  setField('자막 원문', '원본 없이 수정')
  await waitFor(() => expect(calls).toContain('SaveClipEditPlan'), AUTOSAVE)
  // The edit saved without originals; confirming it needs none either (CLIP-57, CLIP-75), but
  // ③'s render does until matching originals are back (CLIP-23).
  await userEvent.click(await screen.findByRole('button', { name: RENDER }))
  const render = await screen.findByRole('button', { name: '렌더하기' })
  expect(render).toBeDisabled()
  await goToStep('영상 다듬기')
  await select()
  await goToStep('영상 완성')
  await waitFor(() => expect(screen.getByRole('button', { name: '렌더하기' })).toBeEnabled())
  expect(calls).not.toContain('StartClipGeneration')
})

it('uses localized selected-cut controls and shows a single audio control', async () => {
  await mount()
  await selectCut()
  await act(async () => initializeI18n('en'))
  expect(screen.getByRole('region', { name: 'Edit cuts and captions' })).toBeInTheDocument()
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
  // 장면 추가 opens the observed-scene chooser; the cut lands after the selected block (CLIP-67).
  await userEvent.click(timeline().getByRole('button', { name: '컷 1' }))
  await userEvent.click(screen.getByRole('button', { name: '장면 추가' }))
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
    focal: { x: 0.3, y: 0.6 },
  })
  expect(screen.queryByRole('dialog', { name: '원본 소스' })).not.toBeInTheDocument()
  expect(screen.queryByRole('dialog', { name: /컷 2/ })).not.toBeInTheDocument()
  await userEvent.click(timeline().getByRole('button', { name: '컷 2 편집' }))
  expect(screen.getByRole('dialog', { name: /컷 2/ })).toBeVisible()
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
  await userEvent.click(screen.getByRole('button', { name: '원본 소스' }))
  await userEvent.click(screen.getByRole('tab', { name: '원본 영상' }))
  const toggle = await screen.findByRole('switch', { name: /source-a.mp4 원본 소리 유지/ })
  await waitFor(() => expect(toggle).toBeEnabled())
  expect(toggle).not.toBeChecked()
  await userEvent.click(toggle)
  await waitFor(() => expect(soundWrites).toHaveLength(1))
  await waitFor(() => expect(screen.getByRole('button', { name: RENDER })).toBeEnabled())
  expect(calls).not.toContain('SaveClipEditPlan')
  await goToStep('영상 생성')
  await clipGuideTo('sources')
  expect(screen.getByRole('switch', { name: /source-a.mp4 원본 소리 유지/ })).toBeChecked()
  await userEvent.click(screen.getByRole('switch', { name: /source-a.mp4 원본 소리 유지/ }))
  await waitFor(() => expect(soundWrites).toHaveLength(2))
  expect(soundWrites.map((w) => w.expectedRevision)).toEqual([1, 2])
  expect(calls).not.toContain('StartClipGeneration')
  expect(calls).not.toContain('QuoteClipGeneration')
  await goToStep('영상 다듬기')
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  await waitFor(() => expect(soundWrites).toHaveLength(3))
  expect(soundWrites[2]).toMatchObject({ expectedRevision: 3, retainOriginalAudio: true })
})

// CLIP-39: ② autosaves as ① does, so 저장 is gone and a dirty draft is not a reason to
// refuse 다시 렌더 — the render flushes the queue itself and runs against what the server
// took, never against a revision the owner has since typed past.
it('confirms a dirty draft by flushing it first, and offers no 저장 anywhere', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  const calls: string[] = []
  await mount({ planWrites: writes, calls })
  await select()
  await waitFor(() => expect(screen.getByRole('button', { name: RENDER })).toBeEnabled())
  await selectText()
  setField('자막 원문', '렌더 직전 수정')
  // Dirty, and no way to save it by hand.
  expect(screen.queryByRole('button', { name: '저장' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: RENDER })).toBeEnabled()
  await userEvent.click(screen.getByRole('button', { name: RENDER }))
  await waitFor(() => expect(calls).toContain('ConfirmClipProject'), AUTOSAVE)
  // The edit reached the server BEFORE the plan was confirmed.
  expect(writes.at(-1)?.plan.elements?.[0].text).toBe('렌더 직전 수정')
  expect(calls.indexOf('SaveClipEditPlan')).toBeLessThan(calls.indexOf('ConfirmClipProject'))
})

it('starts a server render when the retained batch includes an unused original', async () => {
  const project = fixture()
  const batch = create(ClipSourceBatchSchema, {
    id: 'retained',
    projectId: project.id,
    current: true,
    state: 'ready',
    expiresAt: '2099-01-01T00:00:00Z',
    sources: [
      ...project.editing!.sources,
      {
        id: 'unused',
        fingerprint: 'c'.repeat(64),
        filename: 'unused.mp4',
        durationMs: 40000,
        width: 1920,
        height: 1080,
      },
    ].map((source) => ({
      id: source.id,
      state: 'ready',
      availability: 'available',
      actualBytes: 5n,
      retentionExpiresAt: '2099-01-01T00:00:00Z',
      metadata: { ...source, contentType: 'video/mp4', bytes: 5n },
    })),
  })
  project.renderedPlanRevision = 0
  project.result = undefined
  const calls: string[] = []
  const renderStarts: NonNullable<FakeClipsOptions['renderStarts']> = []
  await mount({ projects: [project], retainedBatches: [batch], calls, renderStarts })
  await renderInFinish()
  await waitFor(() => expect(calls).toContain('StartClipRender'))
  expect(renderStarts.at(-1)).toMatchObject({ batchId: 'retained', expectedRevision: 1 })
})

// A plan its own validation refuses never became a job, so the start is not left
// "checking whether it was accepted" with the run view holding the screen.
it('shows a refused server render at once and keeps ③ usable', async () => {
  const calls: string[] = []
  const project = fixture()
  project.renderedPlanRevision = 0
  project.result = undefined
  await mount({
    projects: [project],
    calls,
    renderRefusal: 'CLIP_COMPOSITION_INVALID',
    renderRefusalParams: { element_id: 'narration-2', line: '0', reason: 'rapid_readability' },
  })
  await select()
  await renderInFinish()
  await waitFor(() => expect(calls).toContain('StartClipRender'))
  const finish = within(await screen.findByRole('region', { name: '완성됐어요' }))
  expect(await finish.findByText('영상의 2번째 자막을 확인해 주세요.')).toBeVisible()
  expect(screen.queryByText(/요청의 접수 여부를 확인하는 중이에요/)).not.toBeInTheDocument()
  await waitFor(() => expect(screen.getByRole('button', { name: '렌더하기' })).toBeEnabled())
})

/** 확정하기 carries the owner to ③, which renders the confirmed plan there (CLIP-153): jsdom has
 *  no encoder, so ③ offers 렌더하기 and the server is chosen explicitly. */
async function renderInFinish() {
  const confirmAction = await screen.findByRole('button', { name: '확정하기' })
  await waitFor(() => expect(confirmAction).toBeEnabled())
  await userEvent.click(confirmAction)
  const render = await screen.findByRole('button', { name: '렌더하기' })
  await waitFor(() => expect(render).toBeEnabled())
  await userEvent.click(render)
  await userEvent.click(await screen.findByRole('button', { name: '서버에서 렌더' }))
}

const styleRadio = (name: string) =>
  within(screen.getByRole('radiogroup', { name: '자막 스타일' })).getByRole('radio', {
    name: new RegExp('^' + name),
  })

// CLIP-142, CLIP-143, CLIP-191: the owner gives ONE caption a style outside the project's AI set.
// Only that caption changes — its words and window stay — the AI set is not touched, and the
// draft saves itself.
it('styles one caption outside the AI set and saves only that change', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  const projectWrites: NonNullable<FakeClipsOptions['projectWrites']> = []
  await mount({ planWrites: writes, projectWrites })
  await selectText()
  await userEvent.click(styleRadio('네온 사인'))
  await waitFor(() => expect(writes.at(-1)?.plan.elements?.[0].ownerStyle).toBe('neon'), AUTOSAVE)
  const [styled, other] = writes.at(-1)!.plan.elements!
  const [was, otherWas] = clipTimelineFixture().plan.elements!
  expect(styled).toMatchObject({ text: was.text, startMs: was.startMs, endMs: was.endMs })
  expect(other.ownerStyle).toBeFalsy()
  expect(other.text).toBe(otherWas.text)
  expect(projectWrites).toEqual([])
})

// CDS-100: a size the new style cannot take stays, is reported where the size is set, and holds
// the save and the render until it is corrected; undo steps back out of it.
it('keeps a size the new style cannot take, reports it and holds the render', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  const project = fixture()
  project.editing!.plan.elements![0].ownerSizePx = 64
  await mount({ planWrites: writes, projects: [project] })
  await select()
  await selectText()
  await userEvent.click(styleRadio('키노트'))
  const size = screen.getByLabelText('글자 크기 (px)')
  expect(size).toHaveValue('64')
  expect(size).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByRole('button', { name: RENDER })).toBeDisabled()
  await act(async () => new Promise((resolve) => setTimeout(resolve, 1200)))
  expect(writes.some((write) => write.plan.elements?.[0].ownerStyle === 'keynote')).toBe(false)
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  expect(styleRadio('기본 스타일')).toHaveAttribute('aria-checked', 'true')
  expect(screen.getByLabelText('글자 크기 (px)')).not.toHaveAttribute('aria-invalid')
  await userEvent.click(screen.getByRole('button', { name: '다시 실행' }))
  setField('글자 크기 (px)', '76')
  fireEvent.blur(screen.getByLabelText('글자 크기 (px)'))
  await waitFor(
    () =>
      expect(writes.at(-1)?.plan.elements?.[0]).toMatchObject({
        ownerStyle: 'keynote',
        ownerSizePx: 76,
      }),
    AUTOSAVE,
  )
  await waitFor(() => expect(screen.getByRole('button', { name: RENDER })).toBeEnabled())
})

// CLIP-134: captions are selected by their own identity — two on one cut, one across cuts — and
// a style given to one of them is that caption's alone.
it('selects each caption by its identity, several on one cut and one across cuts', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  const editing = clipNarrationFixture()
  const first = editing.plan.elements!.find((t) => t.instanceId === 'narration-1')!
  editing.plan.elements!.push({
    ...first,
    instanceId: 'narration-3',
    elementId: 'narration-3',
    text: '같은 컷의 둘째 자막',
    startMs: 5500,
    endMs: 7500,
    resolvedStartMs: 5500,
    resolvedEndMs: 7500,
  })
  await mount({ planWrites: writes, projects: [{ ...fixture(), editing }] })
  await selectText('같은 컷의 둘째 자막')
  expect(screen.getByLabelText('자막 원문')).toHaveValue('같은 컷의 둘째 자막')
  await userEvent.click(styleRadio('필름 자막'))
  await waitFor(
    () =>
      expect(
        writes.at(-1)?.plan.elements?.find((t) => t.instanceId === 'narration-3')?.ownerStyle,
      ).toBe('film'),
    AUTOSAVE,
  )
  const saved = writes.at(-1)!.plan.elements!
  expect(saved.find((t) => t.instanceId === 'narration-1')?.ownerStyle).toBeFalsy()
  expect(saved.find((t) => t.instanceId === 'narration-2')?.ownerStyle).toBeFalsy()
  await selectText('컷을 건너가는 자막')
  expect(screen.getByLabelText('자막 원문')).toHaveValue('컷을 건너가는 자막')
  expect(styleRadio('기본 스타일')).toHaveAttribute('aria-checked', 'true')
})

vi.mock('@/entities/clip-preview/ui/useLocalSamples', () => localSamplesForControls)

// Control/store tests use the deterministic compatibility player; actual local Worker/audio
// and bitmap/gesture behavior are exercised in the mounted Chrome harness.
vi.mock('@/features/preview-clip-draft', async (original) => {
  const actual = await original<object>()
  const { ClipDraftPreview } = await import('@/entities/clip-preview')
  return {
    ...actual,
    ClipDraftPreviewPanel: (props: import('react').ComponentProps<typeof ClipDraftPreview>) => (
      <ClipDraftPreview
        {...props}
        localMode={false}
        preview={{
          assets: [],
          canvasWidth: 1080,
          canvasHeight: 1920,
          ready: true,
          updating: false,
          onRetry: () => {},
        }}
      />
    ),
  }
})
