import { useReducer } from 'react'
import userEvent from '@testing-library/user-event'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineReducer, createClipTimeline } from '@/entities/clip-plan'
import { clipTimelineFixture } from '@/test/clip-editing'
import { ClipSceneBlockSheet, type ClipCutSound } from './ClipSceneBlockSheet'

afterEach(cleanup)

/** The real reducer behind the sheet, so an edit lands in the draft the preview draws. */
function Harness({
  sound,
  split = async () => {},
  allowedRates = [1000, 1250, 1500, 2000],
}: {
  sound?: ClipCutSound
  split?: (sourceMs: number) => Promise<void>
  allowedRates?: number[]
}) {
  const state = clipTimelineFixture()
  const [timeline, dispatch] = useReducer(clipTimelineReducer, state.plan, createClipTimeline)
  const cut = timeline.plan.cuts[0]
  return (
    <>
      <output aria-label="range">{`${cut.startMs}-${cut.endMs}`}</output>
      <output aria-label="rate">{cut.playbackRatePermille}</output>
      <button type="button" onClick={() => dispatch({ type: 'seek', timeMs: 4000 })}>
        play to 4 s
      </button>
      <ClipSceneBlockSheet
        open
        onClose={() => {}}
        plan={timeline.plan}
        cut={cut}
        source={{ ...state.sources[0], allowedRatePermille: allowedRates }}
        playheadMs={timeline.timeMs}
        sound={sound}
        footage={{ localSources: [] }}
        change={(edit, group) => dispatch({ type: 'edit', edit, at: Date.now(), group })}
        seek={(timeMs) => dispatch({ type: 'seek', timeMs })}
        endTransaction={() => dispatch({ type: 'endTransaction' })}
        split={split}
      />
    </>
  )
}

it('trims the source range by exact source-time entry, the sheet staying open', () => {
  render(<Harness />)
  expect(screen.getByLabelText('range')).toHaveTextContent('0-10000')
  fireEvent.change(screen.getByLabelText('원본 시작 (초)'), { target: { value: '1.5' } })
  fireEvent.change(screen.getByLabelText('원본 끝 (초)'), { target: { value: '8.25' } })
  expect(screen.getByLabelText('range')).toHaveTextContent('1500-8250')
  expect(screen.getByRole('dialog', { name: /컷 1/ })).toBeInTheDocument()
})

it("splits at the preview's playhead in source time", async () => {
  const split = vi.fn(async () => {})
  render(<Harness split={split} />)
  // At output 0 the playhead stands on the cut's first frame, where nothing can be split.
  expect(screen.getByRole('button', { name: '재생 위치에서 나누기' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: 'play to 4 s' }))
  await userEvent.click(screen.getByRole('button', { name: '재생 위치에서 나누기' }))
  expect(split).toHaveBeenCalledWith(4000)
})

it('offers only the rates the source admits and sets the chosen one', async () => {
  render(<Harness allowedRates={[1000, 1500]} />)
  await userEvent.click(screen.getByRole('combobox', { name: /재생 속도/ }))
  expect(screen.getByRole('option', { name: /^0.5×/ })).toHaveAttribute('aria-disabled', 'true')
  expect(screen.getByRole('option', { name: /^2×/ })).toHaveAttribute('aria-disabled', 'true')
  await userEvent.click(screen.getByRole('option', { name: '1.5×' }))
  expect(screen.getByLabelText('rate')).toHaveTextContent('1500')
})

it("toggles the cut source's original sound, and only states it where it cannot change", async () => {
  const change = vi.fn()
  const { unmount } = render(<Harness sound={{ value: false, change }} />)
  const toggle = screen.getByRole('switch', { name: '원본 소리 유지' })
  expect(toggle).not.toBeChecked()
  await userEvent.click(toggle)
  expect(change).toHaveBeenCalledWith(true)
  unmount()
  render(<Harness sound={{ value: true }} />)
  expect(screen.getByRole('switch', { name: '원본 소리 유지' })).toBeChecked()
  expect(screen.getByRole('switch', { name: '원본 소리 유지' })).toBeDisabled()
})
