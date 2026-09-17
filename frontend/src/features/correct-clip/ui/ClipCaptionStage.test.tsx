import userEvent from '@testing-library/user-event'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { ClipCanvasBox, ClipCaptionFragment, ClipEditableText } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import { ClipCaptionStage } from './ClipCaptionStage'
import { ClipTextControls } from './ClipTextControls'

afterEach(cleanup)

const canvas: ClipCanvasBox = { x: 0, y: 0, width: 1080, height: 1920 }
// The 9:16 safe area CDS-9 names, as the server sends it.
const safeArea: ClipCanvasBox = { x: 64, y: 40, width: 952, height: 1380 }
const fragment = (patch: Partial<ClipCaptionFragment> = {}): ClipCaptionFragment => ({
  instanceId: 'caption-a',
  svg: '<g><text>여기 진짜 좋아요</text></g>',
  box: { x: 300, y: 1200, width: 400, height: 120 },
  fontSize: 72,
  style: 'bold',
  representativeFrame: false,
  ...patch,
})
const caption = (patch: Partial<ClipEditableText> = {}): ClipEditableText => ({
  ...clipTimelineFixture().plan.elements![0],
  instanceId: 'caption-a',
  ...patch,
})

function stage(props: Partial<Parameters<typeof ClipCaptionStage>[0]> = {}) {
  const change = vi.fn()
  render(
    <ClipCaptionStage
      text={caption()}
      fragment={fragment()}
      canvas={canvas}
      safeArea={safeArea}
      frameUrl="blob:source-a"
      frameStartMs={2000}
      change={change}
      {...props}
    />,
  )
  return change
}

it('drags a caption and stops it at the safe area edge, drawing the edge while it moves', () => {
  const change = stage()
  const handle = screen.getByRole('button', { name: '자막을 끌어서 옮기기 (방향키로 미세 조정)' })
  expect(screen.queryByTestId('clip-caption-safe-area')).not.toBeInTheDocument()
  // jsdom gives every element a zero-sized box, so the stage's own width is
  // stubbed: the drag maths is what this test is about, not layout.
  const box = handle.closest('div')!
  vi.spyOn(box, 'getBoundingClientRect').mockReturnValue({ width: 1080, height: 1920 } as DOMRect)
  fireEvent.pointerDown(handle, { clientX: 500, clientY: 1250, pointerId: 1 })
  expect(screen.getByTestId('clip-caption-safe-area')).toBeInTheDocument()
  // Far past the right edge: the caption stops with its box inside the safe area.
  fireEvent.pointerMove(handle, { clientX: 5000, clientY: 1250, pointerId: 1 })
  fireEvent.pointerUp(handle, { pointerId: 1 })
  expect(change).toHaveBeenCalledWith(
    { type: 'text', id: 'caption-a', patch: { ownerPosition: { x: 616, y: 1200 } } },
    undefined,
  )
  expect(screen.queryByTestId('clip-caption-safe-area')).not.toBeInTheDocument()
})

it('nudges a caption with the arrow keys', async () => {
  const change = stage({ text: caption({ ownerPosition: { x: 300, y: 1200 } }) })
  const handle = screen.getByRole('button', { name: '자막을 끌어서 옮기기 (방향키로 미세 조정)' })
  handle.focus()
  await userEvent.keyboard('{ArrowLeft}')
  expect(change).toHaveBeenLastCalledWith(
    { type: 'text', id: 'caption-a', patch: { ownerPosition: { x: 292, y: 1200 } } },
    undefined,
  )
  await userEvent.keyboard('{Shift>}{ArrowUp}{/Shift}')
  expect(change).toHaveBeenLastCalledWith(
    { type: 'text', id: 'caption-a', patch: { ownerPosition: { x: 300, y: 1160 } } },
    undefined,
  )
})

it('still edits over a neutral ground when the source media is not here, and says why', () => {
  stage({ frameUrl: undefined })
  expect(
    screen.getByText(
      '이 컷의 원본이 여기 없어서 화면 대신 빈 배경에 올려요. 글·시간·크기·스타일은 그대로 고칠 수 있어요.',
    ),
  ).toBeInTheDocument()
  // The caption is still placeable: only the frame beneath it is missing.
  expect(
    screen.getByRole('button', { name: '자막을 끌어서 옮기기 (방향키로 미세 조정)' }),
  ).toBeInTheDocument()
})

it('shows a contrast shortfall where the caption is, and blocks nothing', () => {
  stage({
    notices: [
      {
        code: 'composition_contrast',
        elementId: caption().elementId,
        cutId: caption().cutId,
        action: 'notice',
      },
    ],
    language: 'ko',
  })
  expect(screen.getByRole('list', { name: '클립에 반영된 내용' })).toBeInTheDocument()
  // Nothing about the stage disables the caption or its placement.
  expect(
    screen.getByRole('button', { name: '자막을 끌어서 옮기기 (방향키로 미세 조정)' }),
  ).toBeEnabled()
})

it('says a sequence-rendered style is shown as one frame of its motion', () => {
  stage({ fragment: fragment({ representativeFrame: true }) })
  expect(
    screen.getByText('한 장면만 보여 주는 스타일이에요. 실제 영상에서는 움직여요.'),
  ).toBeInTheDocument()
})

it('refuses a size below the role floor at the control and keeps what was typed', async () => {
  const plan = clipTimelineFixture().plan
  const change = vi.fn()
  render(
    <ClipTextControls
      plan={plan}
      text={plan.elements![0]}
      change={change}
      invalid={false}
      captionStyles={['bold', 'film']}
    />,
  )
  const size = screen.getByLabelText('글자 크기 (px)')
  await userEvent.type(size, '12')
  fireEvent.blur(size)
  expect(change).not.toHaveBeenCalled()
  expect(size).toHaveValue('12')
  expect(screen.getByText('64–72 px 사이로 넣어 주세요.')).toBeInTheDocument()
  await userEvent.clear(size)
  await userEvent.type(size, '68')
  fireEvent.blur(size)
  expect(change).toHaveBeenCalledWith(
    { type: 'text', id: plan.elements![0].instanceId, patch: { ownerSizePx: 68 } },
    undefined,
  )
})

it('offers only the styles the project allows, defaulting to the project default', async () => {
  const plan = clipTimelineFixture().plan
  const change = vi.fn()
  render(
    <ClipTextControls
      plan={plan}
      text={plan.elements![0]}
      change={change}
      invalid={false}
      captionStyles={['bold', 'film']}
    />,
  )
  const style = screen.getByLabelText('자막 스타일')
  await userEvent.click(style)
  expect(screen.getByRole('option', { name: '필름 자막' })).toBeInTheDocument()
  expect(screen.queryByRole('option', { name: '네온 사인' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('option', { name: '필름 자막' }))
  expect(change).toHaveBeenCalledWith(
    {
      type: 'text',
      id: plan.elements![0].instanceId,
      patch: { ownerStyle: 'film', ownerSizePx: undefined },
    },
    undefined,
  )
})
