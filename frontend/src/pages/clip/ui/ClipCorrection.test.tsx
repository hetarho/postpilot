import { afterEach, expect, it, vi } from 'vitest'
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
          copyStyles: ['clean'],
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
const selectText = async (name = 'caption a') =>
  userEvent.click(timeline().getByRole('button', { name }))
const setField = (label: string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
const savePlan = async () => userEvent.click(screen.getByRole('button', { name: '저장' }))
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
  expect(screen.getAllByLabelText('원본 시작 (초)')).toHaveLength(1)
  await goToStep('클립 생성')
  expect(screen.queryByRole('button', { name: '다시 렌더' })).not.toBeInTheDocument()
})

it('saves exact milliseconds and selected text with a new optimistic revision', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  setField('원본 시작 (초)', '0.123')
  setField('원본 끝 (초)', '12.345')
  await selectText()
  setField('자막 원문', '오늘 장면')
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0].plan.cuts[0]).toMatchObject({ startMs: 123, endMs: 12345 })
  expect(writes[0].plan.elements![0].text).toBe('오늘 장면')
  expect(writes[0].revision).toBe(1)
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeInTheDocument()
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
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0].plan.cuts.map((c) => c.id)).toEqual(['cut-b', 'cut-a'])
})

it('preserves invalid authored intervals and disables rerender until repaired', async () => {
  const calls: string[] = []
  await mount({ calls })
  await selectText()
  setField('표시 끝 (초)', '12')
  expect(screen.getByLabelText('표시 끝 (초)')).toHaveValue(12)
  expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeDisabled()
  expect(screen.getByText(/이 문구의 내용·위치·시간/)).toBeInTheDocument()
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
  await userEvent.click(screen.getByRole('button', { name: '컷 삭제' }))
  expect(timeline().queryByRole('button', { name: 'caption a' })).not.toBeInTheDocument()
  expect(timeline().getByRole('button', { name: '정확한 고정 문구' })).toBeInTheDocument()
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(1))
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  expect(timeline().getByRole('button', { name: 'caption a' })).toBeInTheDocument()
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(2))
  expect(writes.map((w) => w.revision)).toEqual([1, 2])
})

it('edits fixed global text without requiring legacy campaign facts', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  await selectText('정확한 고정 문구')
  setField('자막 원문', '직접 쓴 문구 <그대로>')
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0].plan.elements!.find((t) => t.instanceId === 'global')).toMatchObject({
    text: '직접 쓴 문구 <그대로>',
    basis: 'whole',
    kind: 'fixed',
  })
})

it('splits and edits rapid phrase timing in exact milliseconds and merges back', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  await selectText()
  setField('자막 원문', '오늘은 구로디지털단지에 와보았는데요')
  await userEvent.click(screen.getByRole('combobox', { name: /^자막 흐름/ }))
  await userEvent.click(screen.getByRole('option', { name: '빠른 구절형' }))
  expect(screen.getByLabelText('구절 1')).toHaveValue('오늘은')
  const ends = screen.getAllByLabelText('컷 안에서 끝 (초)')
  fireEvent.change(ends[1], { target: { value: '0.919' } })
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0].plan.elements![0].phrases![1]).toMatchObject({
    text: '구로디지털단지에',
    startMs: 420,
    endMs: 919,
  })
  await userEvent.click(screen.getByRole('combobox', { name: /^자막 흐름/ }))
  await userEvent.click(screen.getByRole('option', { name: '문장형' }))
  expect(screen.getByLabelText('자막 원문')).toHaveValue('오늘은 구로디지털단지에 와보았는데요')
})

it('keeps the local draft on conflict and reloads only after an explicit discard', async () => {
  await mount({ planSaveConflict: true })
  await selectText()
  setField('자막 원문', '내 수정')
  await savePlan()
  await screen.findByRole('button', { name: '내 편집을 최신 버전에 적용' })
  expect(screen.getByLabelText('자막 원문')).toHaveValue('내 수정')
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeInTheDocument()
})

it('requires matching sources for rerender while text edits and previous video remain usable', async () => {
  const calls: string[] = []
  await mount({ calls })
  await selectText()
  setField('자막 원문', '원본 없이 수정')
  await savePlan()
  await waitFor(() => expect(calls).toContain('SaveClipEditPlan'))
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeDisabled()
  await select()
  await waitFor(() => expect(screen.getByRole('button', { name: '다시 렌더' })).toBeEnabled())
  expect(calls).not.toContain('StartClipGeneration')
})

it('uses localized selected-cut controls and shows a single audio control', async () => {
  await mount()
  await act(async () => initializeI18n('en'))
  expect(screen.getByRole('heading', { name: 'Edit cuts and captions' })).toBeInTheDocument()
  expect(screen.getByLabelText('Source start (seconds)')).toHaveValue(0)
  expect(screen.getAllByLabelText('Original audio (%)')).toHaveLength(1)
})

it('links multiple observed ranges to one item without changing prices or observations', async () => {
  const p = fixture(),
    source = p.editing!.sources[0]
  p.composition = {
    snapshot: { version: 1, body: '<clip version="1"/>', templateId: 'template', legacy: false },
    inputs: {
      values: {},
      items: {
        menu: [
          { id: 'a', values: { name: '밥', price: '8000' } },
          { id: 'b', values: { name: '국', price: '12000' } },
        ],
      },
      associations: [],
    },
  }
  p.observations = {
    status: 'available',
    sources: [
      {
        source,
        segments: [
          { startMs: 0, endMs: 5000, event: '첫 장면', subjects: [], speech: '', quality: '' },
          {
            startMs: 5000,
            endMs: 10000,
            event: '두 번째 장면',
            subjects: [],
            speech: '',
            quality: '',
          },
        ],
      },
    ],
  }
  const before = structuredClone(p.observations),
    writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ projects: [p], planWrites: writes })
  await userEvent.click(screen.getByRole('checkbox', { name: /첫 장면/ }))
  await userEvent.click(screen.getByRole('checkbox', { name: /두 번째 장면/ }))
  await savePlan()
  await waitFor(() => expect(writes).toHaveLength(1))
  expect(writes[0].plan.associations).toHaveLength(2)
  expect(writes[0].plan.associations!.every((a) => a.itemId === 'a')).toBe(true)
  await selectText()
  expect(screen.getByLabelText('자막 원문')).toHaveValue('caption a')
  expect(screen.getByText(/항목 연결이 바뀌었어요/)).toBeInTheDocument()
  expect(p.observations).toEqual(before)
  expect(p.composition.inputs.items.menu.map((i) => i.values.price)).toEqual(['8000', '12000'])
  await userEvent.click(screen.getByRole('button', { name: '문구와 항목이 맞아요' }))
  expect(screen.queryByText(/항목 연결이 바뀌었어요/)).not.toBeInTheDocument()
})

it('snaps range gestures to the output frame grid while precise typing retains milliseconds', async () => {
  await mount()
  fireEvent.change(screen.getByRole('slider', { name: '시작 손잡이 · 원본' }), {
    target: { value: '124' },
  })
  expect(screen.getByLabelText('원본 시작 (초)')).toHaveValue(0.133)
  setField('원본 시작 (초)', '0.124')
  expect(screen.getByLabelText('원본 시작 (초)')).toHaveValue(0.124)
  expect(screen.getByRole('button', { name: '현재 프레임을 시작으로' })).toBeDisabled()
})
