import userEvent from '@testing-library/user-event'
import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { type ClipEditPlan } from '@/entities/clip-plan'
import { ClipTimeline } from './ClipTimeline'

afterEach(cleanup)

/** A plan of `count` adjacent cuts of the same length, with no captions, so the
 *  only thing on the strip is the cut track and its ruler. */
function adjacentCuts(count: number, lengthMs: number): ClipEditPlan {
  const plan = clipTimelineFixture().plan
  const [first] = plan.cuts
  return {
    ...plan,
    elements: [],
    durationMs: count * lengthMs,
    cuts: Array.from({ length: count }, (_, index) => ({
      ...first,
      id: `cut-${index}`,
      sourceId: 'a',
      startMs: 0,
      endMs: lengthMs,
      transitionMs: 0,
      copies: [],
    })),
  }
}

/** The percentage geometry the component actually laid out, read off the style
 *  attribute rather than off pixels jsdom never computes. */
function geometry(elements: HTMLElement[]) {
  return elements.map((el) => ({
    left: Number.parseFloat(el.style.left),
    width: Number.parseFloat(el.style.width),
  }))
}

it('gives every cut label its own cut width, so no label reaches its neighbour', () => {
  render(
    <ClipTimeline plan={adjacentCuts(10, 1000)} timeMs={0} onSelect={vi.fn()} localSources={[]} />,
  )
  const cuts = within(screen.getByRole('list', { name: '영상 컷' })).getAllByRole('listitem')
  expect(cuts).toHaveLength(10)
  // One second-mark per cut, each bounded by the cut it belongs to.
  const ticks = geometry(screen.getAllByText(/ s$/))
  expect(ticks).toHaveLength(10)
  for (const [index, tick] of ticks.entries()) {
    expect(tick.width).toBeCloseTo(10, 6)
    expect(tick.left).toBeCloseTo(index * 10, 6)
    if (index + 1 < ticks.length) {
      expect(tick.left + tick.width).toBeLessThanOrEqual(ticks[index + 1].left + 1e-6)
    }
  }
  // And one readable label per cut, in the cut's own box.
  for (let index = 0; index < 10; index++) {
    expect(within(cuts[index]).getByText(`컷 ${index + 1}`)).toBeVisible()
  }
})

it('drops the label of a cut too narrow to hold one rather than truncating it away', () => {
  // 20 cuts over 5 s: the strip is 400 px, so each cut is 20 px — under the
  // floor a label needs, and the second mark and the cut name are both gone.
  render(
    <ClipTimeline plan={adjacentCuts(20, 250)} timeMs={0} onSelect={vi.fn()} localSources={[]} />,
  )
  const cuts = within(screen.getByRole('list', { name: '영상 컷' })).getAllByRole('listitem')
  expect(cuts).toHaveLength(20)
  expect(screen.queryByText(/ s$/)).not.toBeInTheDocument()
  expect(screen.queryByText('컷 1')).not.toBeInTheDocument()
  // The cut is still there to select; only its label is not.
  expect(within(cuts[0]).getByRole('button')).toBeInTheDocument()
})

it('draws every thumbnail from the first render, with no selection and no playhead', () => {
  const plan = adjacentCuts(4, 2000)
  const { container } = render(
    <ClipTimeline
      plan={plan}
      timeMs={0}
      onSelect={vi.fn()}
      localSources={[{ fingerprint: 'a'.repeat(64), url: 'blob:source-a' }]}
    />,
  )
  const videos = container.querySelectorAll('video')
  expect(videos).toHaveLength(4)
  expect(videos[0]).toHaveAttribute('src', 'blob:source-a#t=0')
})

it('keeps the film glyph where the cut has no playable source', () => {
  const { container } = render(
    <ClipTimeline plan={adjacentCuts(3, 2000)} timeMs={0} onSelect={vi.fn()} localSources={[]} />,
  )
  expect(container.querySelectorAll('video')).toHaveLength(0)
  expect(container.querySelectorAll('svg.size-6')).toHaveLength(3)
})

it('heads the timeline with undo and redo as icon controls named for what they do', async () => {
  const undo = vi.fn(),
    redo = vi.fn()
  render(
    <ClipTimeline
      plan={adjacentCuts(3, 2000)}
      timeMs={0}
      onSelect={vi.fn()}
      localSources={[]}
      history={{ undo, redo, canUndo: true, canRedo: false }}
    />,
  )
  const undoButton = screen.getByRole('button', { name: '실행 취소' })
  expect(undoButton.className).toContain('pointer-coarse:size-11')
  expect(undoButton).toHaveTextContent('')
  await userEvent.click(undoButton)
  expect(undo).toHaveBeenCalled()
  expect(screen.getByRole('button', { name: '다시 실행' })).toBeDisabled()
  expect(redo).not.toHaveBeenCalled()
})
