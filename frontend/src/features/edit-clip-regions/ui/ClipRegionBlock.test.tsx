import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ClipProjectRegion } from '@/entities/clip-project'
import { ClipRegionBlock } from './ClipRegionBlock'

afterEach(cleanup)

const slot = (n: number, text = '', extra = {}) => ({
  id: `project-intro-${n}`,
  instruction: '',
  text,
  instructionEdited: false,
  ownerFixed: false,
  bound: false,
  ...extra,
})
const region = (extra: Partial<ClipProjectRegion> = {}): ClipProjectRegion => ({
  enabled: true,
  slots: [
    slot(1, '성수 로컬 가이드', { ownerFixed: true }),
    slot(2, '', { ownerFixed: true }),
    slot(3, '대표 메뉴', { bound: true }),
    slot(4, '생성된 문구'),
    slot(5),
    slot(6, '남은 문구', { ownerFixed: true }),
  ],
  ...extra,
})
function mount(props: Partial<Parameters<typeof ClipRegionBlock>[0]> = {}) {
  const handlers = { onEnabled: vi.fn(), onSlot: vi.fn(), onMove: vi.fn() }
  render(
    <ClipRegionBlock
      kind="intro"
      region={region()}
      capacity={5}
      presetName="매거진 커버"
      canvas={{ width: 1080, height: 1920 }}
      drawable
      errors={{}}
      notices={(id) =>
        id === 'project-intro-6'
          ? [{ code: 'region_line_surplus', cutId: '', elementId: id, action: 'removal' }]
          : []
      }
      readOnly={false}
      {...handlers}
      {...props}
    />,
  )
  return handlers
}
const line = (n: number) => within(screen.getAllByRole('listitem')[n - 1])

// CLIP-186, CLIP-187: each active slot reads its state, and an owner's deliberate blank is told
// apart from a slot still waiting for words.
it('names each active slot’s state beside its instruction and final text', () => {
  mount()
  const block = within(screen.getByRole('region', { name: '인트로' }))
  expect(block.getByText('매거진 커버')).toBeVisible()
  expect(line(1).getByText('직접 입력')).toBeVisible()
  expect(line(1).getByLabelText('1번째 줄 문구')).toHaveValue('성수 로컬 가이드')
  expect(line(2).getByText('비워 둠')).toBeVisible()
  expect(line(3).getByText('답변에서')).toBeVisible()
  expect(line(4).getByText('생성됨')).toBeVisible()
  expect(line(5).getByText('생성 대기')).toBeVisible()
  expect(line(1).getByLabelText('1번째 줄 들어갈 내용')).toHaveValue('')
  // No per-slot AI action: the words come from typing or the approved storyline calls.
  expect(block.queryByRole('button', { name: /생성|만들기/ })).not.toBeInTheDocument()
})

it('hands an instruction and a final text back as separate edits', () => {
  const { onSlot } = mount()
  fireEvent.change(line(2).getByLabelText('2번째 줄 들어갈 내용'), {
    target: { value: '영업 시간' },
  })
  expect(onSlot).toHaveBeenLastCalledWith('project-intro-2', { instruction: '영업 시간' })
  fireEvent.change(line(2).getByLabelText('2번째 줄 문구'), { target: { value: '저녁 영업' } })
  expect(onSlot).toHaveBeenLastCalledWith('project-intro-2', { text: '저녁 영업' })
})

// CLIP-189: words past the preset's slots stay readable and editable with their notice, and
// move into an active slot.
it('keeps unused words with their notice and moves them into a slot', async () => {
  const { onMove } = mount()
  const unused = within(screen.getByRole('region', { name: '쓰지 않는 문구' }))
  expect(unused.getByLabelText('쓰지 않는 문구 1')).toHaveValue('남은 문구')
  expect(
    unused.getByText(
      '선택한 인트로·아웃트로 디자인이 담을 수 있는 줄 수를 넘겨서 마지막 줄은 넣지 않았어요.',
    ),
  ).toBeVisible()
  await userEvent.click(unused.getByRole('button', { name: '쓰지 않는 문구 1 옮기기' }))
  await userEvent.click(await screen.findByRole('menuitem', { name: '2번째 줄로 옮기기' }))
  expect(onMove).toHaveBeenCalledWith('project-intro-6', 'project-intro-2')
})

// CLIP-187: an enabled region with nothing to draw says so instead of reading as included.
it('says when an enabled region draws nothing', () => {
  mount({ drawable: false })
  expect(screen.getByText('아직 영상에 보일 문구가 없어요.')).toBeVisible()
})

// CDS-64: a refused slot names its field and keeps the refused words in it.
it('binds a refusal to its field and keeps the refused text', () => {
  mount({ errors: { 'project-intro-1': '이 줄에 다 들어가지 않아요. 줄여 주세요.' } })
  const field = line(1).getByLabelText('1번째 줄 문구')
  expect(field).toHaveAttribute('aria-invalid', 'true')
  expect(field).toHaveAccessibleDescription('이 줄에 다 들어가지 않아요. 줄여 주세요.')
  expect(field).toHaveValue('성수 로컬 가이드')
})

// CLIP-111, CLIP-179: a block that is off says so, draws no fields and turns back on.
it('turns a region off and on without losing its words', async () => {
  const { onEnabled } = mount({ region: region({ enabled: false }) })
  expect(screen.getByText('사용 안 함')).toBeVisible()
  expect(screen.queryByLabelText('1번째 줄 문구')).not.toBeInTheDocument()
  // The switch is a native control: Tab reaches it and Space flips it.
  await userEvent.tab()
  expect(screen.getByRole('switch', { name: '인트로 사용' })).toHaveFocus()
  await userEvent.keyboard(' ')
  expect(onEnabled).toHaveBeenCalledWith(true)
  onEnabled.mockClear()
  await userEvent.click(screen.getByRole('switch', { name: '인트로 사용' }))
  expect(onEnabled).toHaveBeenCalledWith(true)
})

// CLIP-160, CLIP-179: running and finalized projects read the slots without edit actions.
it('reads the slots without edit actions when read-only', () => {
  mount({ readOnly: true })
  expect(screen.queryByRole('switch')).not.toBeInTheDocument()
  expect(line(1).getByLabelText('1번째 줄 문구')).toHaveAttribute('readonly')
  expect(line(1).getByLabelText('1번째 줄 들어갈 내용')).toHaveAttribute('readonly')
  expect(screen.queryByRole('button', { name: '쓰지 않는 문구 1 옮기기' })).not.toBeInTheDocument()
})
