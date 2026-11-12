import { useReducer } from 'react'
import userEvent from '@testing-library/user-event'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import {
  clipTimelineReducer,
  createClipTimeline,
  type ClipEditPlan,
  type ClipEditingState,
} from '@/entities/clip-plan'
import { clipNarrationFixture, clipTimelineFixture } from '@/test/clip-editing'
import { clipSceneBlocks, sceneBlockOf } from '../model/scene-blocks'
import { ClipSceneBlocks } from './ClipSceneBlocks'

afterEach(cleanup)

/** The real reducer behind the list, so a selection seeks and an edit undoes as in ②. */
function Harness({
  state,
  intro,
  outro,
}: {
  state: ClipEditingState
  intro?: string
  outro?: string
}) {
  const [timeline, dispatch] = useReducer(clipTimelineReducer, state.plan, createClipTimeline)
  return (
    <>
      <output aria-label="playhead">{timeline.timeMs}</output>
      <button type="button" onClick={() => dispatch({ type: 'seek', timeMs: 12000 })}>
        play on
      </button>
      <ClipSceneBlocks
        plan={timeline.plan}
        sources={state.sources}
        selection={timeline.selection}
        timeMs={timeline.timeMs}
        onSelect={(selection) => dispatch({ type: 'select', selection })}
        change={(edit, group) => dispatch({ type: 'edit', edit, at: Date.now(), group })}
        footage={{ localSources: [] }}
        intro={intro && <p>{intro}</p>}
        outro={outro && <p>{outro}</p>}
        onOpen={(selection) => dispatch({ type: 'select', selection })}
        onMove={(id, delta) => {
          const from = timeline.plan.cuts.findIndex((c) => c.id === id)
          const to = from + delta
          if (from < 0 || to < 0 || to >= timeline.plan.cuts.length) return false
          dispatch({ type: 'edit', edit: { type: 'move', from, to }, at: Date.now() })
          return true
        }}
        history={{
          undo: () => dispatch({ type: 'undo' }),
          redo: () => dispatch({ type: 'redo' }),
          canUndo: !!timeline.past.length,
          canRedo: !!timeline.future.length,
        }}
      />
    </>
  )
}
const blocks = () =>
  within(screen.getByRole('region', { name: '장면 블록' }))
    .queryAllByRole('listitem')
    .filter((item) => item.dataset.sceneBlock === 'cut')

describe('sceneBlockOf', () => {
  const cuts = [
    { startMs: 0, endMs: 10000 },
    { startMs: 9800, endMs: 19800 },
  ]
  it('puts a start inside a cut under it, a boundary under the later cut and an early one first', () => {
    expect(sceneBlockOf(4000, cuts)).toBe(0)
    expect(sceneBlockOf(9800, cuts)).toBe(1)
    expect(sceneBlockOf(-50, cuts)).toBe(0)
    expect(sceneBlockOf(25000, cuts)).toBe(1)
    expect(sceneBlockOf(0, [])).toBe(-1)
  })
})

it('stands one block per cut in output order with its frame ground, interval, source and sound', () => {
  const state = clipTimelineFixture()
  state.plan.cuts[1].playbackRatePermille = 1500
  render(<Harness state={state} />)
  const [first, second] = blocks()
  expect(within(first).getByRole('button', { name: '컷 1' })).toBeInTheDocument()
  expect(first).toHaveTextContent('영상 0–10 s')
  expect(first).toHaveTextContent(`${state.sources[0].filename} · 원본 0–10 s`)
  // 1x carries no rate badge; any other rate is stated in text.
  expect(first).not.toHaveTextContent('배속')
  expect(second).toHaveTextContent('1.5배속')
  expect(first).toHaveTextContent(/원본 소리 (켬|끔)/)
})

it('groups captions and spoken segments by where they start, a crossing caption under its first cut', () => {
  const state = clipNarrationFixture()
  state.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      {
        id: 'spoken-1',
        text: '둘째 컷 대본',
        textRevision: 1,
        inputHash: 'h',
        startMs: 9800,
        endMs: 12000,
      },
    ],
  }
  const grouped = clipSceneBlocks(state.plan)
  expect(grouped.map((b) => b.captions.map((c) => c.instanceId))).toEqual([
    ['narration-1', 'narration-2'],
    [],
  ])
  expect(grouped.map((b) => b.spoken.map((s) => s.id))).toEqual([[], ['spoken-1']])
  render(<Harness state={state} />)
  const [first, second] = blocks()
  expect(within(first).getByDisplayValue('컷을 건너가는 자막')).toBeInTheDocument()
  expect(within(second).getByDisplayValue('둘째 컷 대본')).toBeInTheDocument()
})

it('edits a caption in place as one undoable edit', async () => {
  render(<Harness state={clipTimelineFixture()} />)
  const field = screen.getByDisplayValue('caption a')
  fireEvent.change(field, { target: { value: 'caption a 고침' } })
  expect(screen.getByDisplayValue('caption a 고침')).toBe(field)
  await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
  expect(screen.getByDisplayValue('caption a')).toBe(field)
})

it('stands the intro first, the outro last and the disclosure badge after it with its interval', () => {
  render(<Harness state={clipTimelineFixture()} intro="인트로 문구" outro="아웃트로 문구" />)
  const items = within(screen.getByRole('region', { name: '장면 블록' })).getAllByRole('listitem')
  expect(items.map((item) => item.dataset.sceneBlock)).toEqual([
    'intro',
    'cut',
    'cut',
    'outro',
    'badge',
  ])
  expect(items[0]).toHaveTextContent('인트로 문구')
  expect(items[3]).toHaveTextContent('아웃트로 문구')
  expect(items[4]).toHaveTextContent('정확한 고정 문구')
  expect(items[4]).toHaveTextContent('영상 0–19.8 s')
})

it('seeks to a selected block and marks it, offering its own sheet without opening anything', async () => {
  render(<Harness state={clipTimelineFixture()} />)
  await userEvent.click(screen.getByRole('button', { name: '컷 2' }))
  expect(screen.getByLabelText('playhead')).toHaveTextContent('9800')
  const [, second] = blocks()
  expect(second).toHaveAttribute('data-selected', 'true')
  expect(within(second).getByRole('button', { name: '컷 2' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  expect(within(second).getByRole('button', { name: '컷 2 편집' })).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('marks the block playback reaches without moving focus', async () => {
  render(<Harness state={clipTimelineFixture()} />)
  const field = screen.getByDisplayValue('caption a')
  field.focus()
  expect(blocks()[0]).toHaveAttribute('aria-current', 'time')
  fireEvent.click(screen.getByRole('button', { name: 'play on' }))
  expect(blocks()[1]).toHaveAttribute('aria-current', 'time')
  expect(blocks()[0]).not.toHaveAttribute('aria-current')
  expect(document.activeElement).toBe(field)
})

// A plan shape the server may return with no cut keeps the list readable.
it('renders no cut block for an empty plan', () => {
  const state = clipTimelineFixture()
  state.plan = { durationMs: 0, cuts: [] } as ClipEditPlan
  render(<Harness state={state} />)
  expect(blocks()).toEqual([])
})

describe('reordering blocks', () => {
  const order = () => blocks().map((block) => block.dataset.cutId)
  const announced = (text: string) =>
    expect(screen.getByText(text)).toHaveAttribute('role', 'status')

  it('moves a block by its phone move control, announced and undone as one edit', async () => {
    const state = clipTimelineFixture()
    const [a, b] = state.plan.cuts.map((cut) => cut.id)
    render(<Harness state={state} />)
    expect(screen.getByRole('button', { name: '컷 1 위로' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: '컷 1 아래로' }))
    expect(order()).toEqual([b, a])
    announced('컷 1을 2개 중 2번째로 옮겼어요')
    // The control that moved it keeps focus in the block's new place.
    expect(document.activeElement).toBe(
      within(blocks()[1]).getByRole('button', { name: '컷 2 위로' }),
    )
    await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
    expect(order()).toEqual([a, b])
  })

  it('moves the focused block with Alt+Arrow keys, leaving a text field its own keys', async () => {
    const state = clipTimelineFixture()
    const [a, b] = state.plan.cuts.map((cut) => cut.id)
    render(<Harness state={state} />)
    const select = screen.getByRole('button', { name: '컷 2' })
    select.focus()
    await userEvent.keyboard('{Alt>}{ArrowUp}{/Alt}')
    expect(order()).toEqual([b, a])
    expect(document.activeElement).toBe(screen.getByRole('button', { name: '컷 1' }))
    announced('컷 2을 2개 중 1번째로 옮겼어요')
    screen.getByDisplayValue('caption a').focus()
    await userEvent.keyboard('{Alt>}{ArrowDown}{/Alt}')
    expect(order()).toEqual([b, a])
    await userEvent.click(screen.getByRole('button', { name: '실행 취소' }))
    expect(order()).toEqual([a, b])
  })

  it('moves a block dropped on another by its drag handle', () => {
    const state = clipTimelineFixture()
    const [a, b] = state.plan.cuts.map((cut) => cut.id)
    render(<Harness state={state} />)
    fireEvent.dragStart(screen.getByRole('button', { name: '컷 1 끌어서 옮기기' }))
    fireEvent.dragOver(blocks()[1])
    fireEvent.drop(blocks()[1])
    expect(order()).toEqual([b, a])
    announced('컷 1을 2개 중 2번째로 옮겼어요')
  })

  it('offers no reorder control on a plan that is only read', () => {
    const state = clipTimelineFixture()
    render(
      <ClipSceneBlocks
        plan={state.plan}
        sources={state.sources}
        timeMs={0}
        onSelect={() => {}}
        onMove={() => true}
        change={() => {}}
        footage={{ localSources: [] }}
        readOnly
      />,
    )
    expect(screen.queryByRole('button', { name: '컷 1 아래로' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '컷 1 끌어서 옮기기' })).not.toBeInTheDocument()
  })
})
