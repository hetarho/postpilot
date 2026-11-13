import { useReducer, useState } from 'react'
import userEvent from '@testing-library/user-event'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import {
  clipTimelineReducer,
  createClipTimeline,
  type ClipCanvasBox,
  type ClipCaptionFragment,
  type ClipEditableText,
} from '@/entities/clip-plan'
import { CLIP_CAPTION_STYLES } from '@/entities/clip-design'
import { normalizeAppFailure } from '@/shared/api'
import { clipTimelineFixture } from '@/test/clip-editing'
import { ClipCaptionSheet } from './ClipCaptionSheet'
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

it('selects and edits the main preview overlay without drawing a second caption or source frame', async () => {
  const onSelect = vi.fn(),
    onPositionPreview = vi.fn()
  const change = stage({ overlayOnly: true, selected: false, onSelect, onPositionPreview })
  const handle = screen.getByRole('button', { name: '미리보기 자막 선택: caption a' })
  expect(document.querySelector('video')).not.toBeInTheDocument()
  expect(document.querySelector('[data-caption]')).not.toBeInTheDocument()
  expect(screen.queryByText('자막 배치')).not.toBeInTheDocument()
  await userEvent.click(handle)
  expect(onSelect).toHaveBeenCalled()
  expect(change).not.toHaveBeenCalled()
  handle.focus()
  await userEvent.keyboard('{ArrowRight}')
  expect(change).toHaveBeenCalledExactlyOnceWith(
    { type: 'text', id: 'caption-a', patch: { ownerPosition: { x: 308, y: 1200 } } },
    undefined,
  )
  expect(onPositionPreview).toHaveBeenLastCalledWith()
})

it('cancels main-preview placement on Escape without saving its temporary position', () => {
  const onPositionPreview = vi.fn()
  const change = stage({ overlayOnly: true, onPositionPreview })
  const handle = screen.getByRole('button', { name: /자막을 끌어서 옮기기/ })
  vi.spyOn(handle.closest('div')!, 'getBoundingClientRect').mockReturnValue({
    width: 1080,
    height: 1920,
  } as DOMRect)
  fireEvent.pointerDown(handle, { clientX: 500, clientY: 1250, pointerId: 1 })
  fireEvent.pointerMove(handle, { clientX: 600, clientY: 1250, pointerId: 1 })
  expect(onPositionPreview).toHaveBeenLastCalledWith({ instanceId: 'caption-a', x: 400, y: 1200 })
  fireEvent.keyDown(document, { key: 'Escape' })
  fireEvent.pointerUp(handle, { pointerId: 1 })
  expect(change).not.toHaveBeenCalled()
  expect(onPositionPreview).toHaveBeenLastCalledWith()
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

function controls(props: Partial<Parameters<typeof ClipTextControls>[0]> = {}) {
  const plan = clipTimelineFixture().plan
  const change = vi.fn()
  const view = render(
    <ClipTextControls
      plan={plan}
      text={plan.elements![0]}
      change={change}
      invalid={false}
      captionStyles={['bold', 'film']}
      {...props}
    />,
  )
  return { change, plan, view }
}
const styles = () => screen.getByRole('radiogroup', { name: '자막 스타일' })

// CLIP-142, CLIP-143: the owner picks from EVERY approved style, the AI set notwithstanding,
// and the new style keeps the size and place the owner set (CDS-100).
it('offers every approved style whatever the AI set, keeping the owner size', async () => {
  const plan = clipTimelineFixture().plan
  const { change } = controls({ text: { ...plan.elements![0], ownerSizePx: 68 } })
  expect(within(styles()).getByRole('radio', { name: '기본 스타일' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  expect(within(styles()).getAllByRole('radio')).toHaveLength(CLIP_CAPTION_STYLES.length + 1)
  await userEvent.click(within(styles()).getByRole('radio', { name: '네온 사인' }))
  expect(change).toHaveBeenCalledWith(
    { type: 'text', id: plan.elements![0].instanceId, patch: { ownerStyle: 'neon' } },
    undefined,
  )
})

// CDS-83: each tile is the renderer's own drawing of its style; a sequence-rendered one says it
// moves, and without the drawings the tiles still choose by name.
it('draws each style from the renderer and marks the animated ones', () => {
  const sample = (style: string, representativeFrame: boolean): ClipCaptionFragment => ({
    ...fragment({ style, representativeFrame }),
    instanceId: `sample-${style}`,
    svg: `<g data-sample="${style}"></g>`,
    box: { x: 0, y: 0, width: 400, height: 120 },
  })
  controls({ styleSamples: [sample('neon', true), sample('film', false)] })
  const neon = within(styles()).getByRole('radio', { name: /네온 사인/ })
  expect(neon.querySelector('[data-sample="neon"]')).toBeInTheDocument()
  expect(neon.querySelector('svg')?.parentElement).toHaveClass('bg-media-canvas-bg')
  expect(within(neon).getByText('움직이는 스타일')).toBeInTheDocument()
  const film = within(styles()).getByRole('radio', { name: '필름 자막' })
  expect(within(film).queryByText('움직이는 스타일')).not.toBeInTheDocument()
  cleanup()
  controls({ samplesUnavailable: true })
  expect(
    screen.getByText('스타일 그림을 불러오지 못했어요. 이름으로 고를 수 있어요.'),
  ).toBeInTheDocument()
  expect(within(styles()).getByRole('radio', { name: '네온 사인' })).toBeEnabled()
})

// CDS-100: a size the new style cannot take is kept and reported where the size is set, and
// the draft holds it until it is corrected; nothing clears it.
it('reports a kept size the new style cannot take, without resetting it', () => {
  const plan = clipTimelineFixture().plan
  const { change } = controls({
    text: { ...plan.elements![0], ownerStyle: 'keynote', ownerSizePx: 64 },
  })
  const size = screen.getByLabelText('글자 크기 (px)')
  expect(size).toHaveValue('64')
  expect(size).toHaveAttribute('aria-invalid', 'true')
  expect(size).toHaveAccessibleDescription('72–84 px 사이로 넣어 주세요.')
  expect(change).not.toHaveBeenCalled()
})

it('moves the style with the arrow keys', async () => {
  const plan = clipTimelineFixture().plan
  const { change } = controls()
  within(styles()).getByRole('radio', { name: '기본 스타일' }).focus()
  await userEvent.keyboard('{ArrowRight}')
  expect(change).toHaveBeenLastCalledWith(
    {
      type: 'text',
      id: plan.elements![0].instanceId,
      patch: { ownerStyle: CLIP_CAPTION_STYLES[0] },
    },
    undefined,
  )
})

// A save refused on this caption is said on its sheet; another caption's is not.
it('says a refusal on its own caption only', () => {
  const plan = clipTimelineFixture().plan
  const refusal = (elementId: string) =>
    normalizeAppFailure({
      reason: 'CLIP_COMPOSITION_INVALID',
      params: { element_id: elementId, line: '0', reason: 'copy_limit' },
    })
  controls({ failure: refusal('someone-else') })
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  cleanup()
  controls({ failure: refusal(plan.elements![0].elementId) })
  expect(screen.getByRole('alert')).toBeInTheDocument()
})

// CLIP-55: an undo or a redo moves the size, and the field shows the size the draft holds.
it('follows the size the draft holds after an undo', () => {
  const plan = clipTimelineFixture().plan
  const text = { ...plan.elements![0], ownerSizePx: 70 }
  const { view, change } = controls({ text })
  expect(screen.getByLabelText('글자 크기 (px)')).toHaveValue('70')
  view.rerender(
    <ClipTextControls
      plan={plan}
      text={{ ...text, ownerSizePx: undefined }}
      change={change}
      invalid={false}
      captionStyles={['bold', 'film']}
    />,
  )
  expect(screen.getByLabelText('글자 크기 (px)')).toHaveValue('')
})

it("opens a caption's own sheet over a frame from the cut it starts in, with its interval and style", () => {
  const state = clipTimelineFixture()
  const [first, second] = state.plan.cuts
  // A caption opening past the transition into the second cut is placed over that cut's source.
  const text = { ...state.plan.elements![1], startMs: 400, resolvedStartMs: 10200 }
  render(
    <ClipCaptionSheet
      open
      onClose={() => {}}
      plan={state.plan}
      text={text}
      stage={{ canvas, safeArea, fragment: fragment({ instanceId: text.instanceId }) }}
      styles={{ samples: [] }}
      footage={{
        localSources: [
          { fingerprint: first.fingerprint, url: 'blob:first' },
          { fingerprint: second.fingerprint, url: 'blob:second' },
        ],
      }}
      change={vi.fn()}
    />,
  )
  const sheet = within(screen.getByRole('dialog', { name: '선택한 문구' }))
  const frame = document.body.querySelector('video')
  expect(frame?.getAttribute('src')).toMatch(/^blob:second#t=/)
  expect(
    sheet.getByRole('button', { name: '자막을 끌어서 옮기기 (방향키로 미세 조정)' }),
  ).toBeInTheDocument()
  expect(sheet.getByRole('button', { name: '문구 삭제' })).toBeInTheDocument()
  const dialog = screen.getByRole('dialog', { name: '선택한 문구' })
  const body = sheet.getByRole('radiogroup', { name: '자막 스타일' }).closest('.overflow-y-auto')!
  expect(body).not.toContainElement(frame)
  expect(dialog).toContainElement(frame)
  expect(
    sheet
      .getByRole('radiogroup', { name: '자막 스타일' })
      .compareDocumentPosition(sheet.getByLabelText('글자 크기 (px)')) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy()
  expect(
    sheet
      .getByLabelText('글자 크기 (px)')
      .compareDocumentPosition(sheet.getByLabelText('자막 원문')) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy()
})

it('keeps an invalid typed size while choosing another style', async () => {
  const { change } = controls()
  const size = screen.getByLabelText('글자 크기 (px)')
  await userEvent.type(size, '12')
  await userEvent.click(within(styles()).getByRole('radio', { name: '네온 사인' }))
  expect(size).toHaveValue('12')
  expect(size).toHaveAttribute('aria-invalid', 'true')
  expect(change).toHaveBeenLastCalledWith(
    expect.objectContaining({ patch: { ownerStyle: 'neon' } }),
    undefined,
  )
})

it('reveals the focused caption field in the sheet body after keyboard reflow without replacing it', async () => {
  const state = clipTimelineFixture()
  const originalHeight = window.innerHeight
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: 800 })
  try {
    render(
      <ClipCaptionSheet
        open
        onClose={() => {}}
        plan={state.plan}
        text={state.plan.elements![0]}
        stage={{ canvas, safeArea, fragment: fragment() }}
        styles={{ samples: [] }}
        footage={{ localSources: [] }}
        change={vi.fn()}
      />,
    )
    const field = screen.getByLabelText<HTMLTextAreaElement>('자막 원문')
    const body = field.closest<HTMLElement>('.overflow-y-auto')!
    vi.spyOn(body, 'getBoundingClientRect').mockReturnValue({ top: 180, bottom: 420 } as DOMRect)
    vi.spyOn(field, 'getBoundingClientRect').mockReturnValue({ top: 400, bottom: 500 } as DOMRect)
    field.focus()
    field.setSelectionRange(3, 3)
    const value = field.value
    act(() => {
      Object.defineProperty(window, 'innerHeight', { configurable: true, value: 480 })
      fireEvent(window, new Event('resize'))
    })
    await waitFor(() => expect(body.scrollTop).toBeGreaterThan(0))
    expect(screen.getByLabelText('자막 원문')).toBe(field)
    expect(document.activeElement).toBe(field)
    expect(field.value).toBe(value)
    expect(field.selectionStart).toBe(3)
    expect(field.selectionEnd).toBe(3)
  } finally {
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: originalHeight })
  }
})

it('retains the manually checked style after closing and reopening the caption sheet', async () => {
  function Harness() {
    const [timeline, dispatch] = useReducer(
      clipTimelineReducer,
      clipTimelineFixture().plan,
      createClipTimeline,
    )
    const [open, setOpen] = useState(true)
    return (
      <>
        <button onClick={() => setOpen(true)}>Open caption</button>
        <ClipCaptionSheet
          open={open}
          onClose={() => setOpen(false)}
          plan={timeline.plan}
          text={timeline.plan.elements![0]}
          stage={{ canvas, safeArea, fragment: fragment() }}
          styles={{ samples: [] }}
          footage={{ localSources: [] }}
          change={(edit) => dispatch({ type: 'edit', edit, at: 0 })}
        />
      </>
    )
  }
  render(<Harness />)
  const film = () =>
    screen
      .getByRole('dialog', { name: '선택한 문구' })
      .querySelector<HTMLButtonElement>('[data-style="film"]')!
  await userEvent.click(film())
  expect(film()).toHaveAttribute('aria-checked', 'true')
  await userEvent.click(screen.getByRole('button', { name: '닫기', exact: true }))
  await userEvent.click(screen.getByRole('button', { name: 'Open caption' }))
  expect(film()).toHaveAttribute('aria-checked', 'true')
})

it('bounds a tall focused textarea on keyboard resize without hiding its selection near the start', async () => {
  const state = clipTimelineFixture()
  const originalHeight = window.innerHeight
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: 800 })
  try {
    const view = render(
      <ClipCaptionSheet
        open
        onClose={() => {}}
        plan={state.plan}
        text={{ ...state.plan.elements![0], text: '긴 자막\n'.repeat(100) }}
        stage={{ canvas, safeArea, fragment: fragment() }}
        styles={{ samples: [] }}
        footage={{ localSources: [] }}
        change={vi.fn()}
      />,
    )
    const field = screen.getByLabelText<HTMLTextAreaElement>('자막 원문')
    const body = field.closest<HTMLElement>('.overflow-y-auto')!
    const original = { maxHeight: '700px', overflowY: 'hidden', overscrollBehaviorY: '' }
    Object.assign(field.style, original)
    vi.spyOn(body, 'getBoundingClientRect').mockReturnValue({ top: 180, bottom: 420 } as DOMRect)
    vi.spyOn(field, 'getBoundingClientRect').mockImplementation(
      () =>
        ({
          top: 200 - body.scrollTop,
          bottom: 200 - body.scrollTop + Math.min(600, parseFloat(field.style.maxHeight) || 600),
          height: Math.min(600, parseFloat(field.style.maxHeight) || 600),
        }) as DOMRect,
    )
    field.focus()
    field.setSelectionRange(2, 4, 'backward')
    const value = field.value
    act(() => {
      Object.defineProperty(window, 'innerHeight', { configurable: true, value: 480 })
      fireEvent(window, new Event('resize'))
    })
    await waitFor(() => expect(field.style.maxHeight).toBe('224px'))
    expect(field.style.overflowY).toBe('auto')
    expect(field.getBoundingClientRect().top).toBeGreaterThanOrEqual(188)
    expect(field.getBoundingClientRect().bottom).toBeLessThanOrEqual(412)
    expect(document.activeElement).toBe(field)
    expect(field.value).toBe(value)
    expect(field.selectionStart).toBe(2)
    expect(field.selectionEnd).toBe(4)
    expect(field.selectionDirection).toBe('backward')
    expect(field.scrollTop).toBe(0)
    view.unmount()
    expect(field.style.maxHeight).toBe(original.maxHeight)
    expect(field.style.overflowY).toBe(original.overflowY)
    expect(field.style.overscrollBehaviorY).toBe(original.overscrollBehaviorY)
  } finally {
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: originalHeight })
  }
})
