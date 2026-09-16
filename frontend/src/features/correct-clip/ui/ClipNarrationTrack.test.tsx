import userEvent from '@testing-library/user-event'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipNarrationFixture } from '@/test/clip-editing'
import { ClipTimeline } from './ClipTimeline'
import { ClipTextControls } from './ClipTextControls'

afterEach(cleanup)

it('shows the narration in its own lane and adds a caption at the playhead', async () => {
  const plan = clipNarrationFixture().plan
  const onAddCaption = vi.fn()
  render(
    <ClipTimeline
      plan={plan}
      timeMs={6000}
      onSelect={vi.fn()}
      onAddCaption={onAddCaption}
      localSources={[]}
    />,
  )
  const track = screen.getByRole('list', { name: '자막 트랙' })
  const bars = within(track).getAllByRole('button')
  expect(bars.map((bar) => bar.textContent)).toEqual(['첫 자막', '컷을 건너가는 자막'])
  // The caption that crosses the cut boundary is one bar on the output timeline.
  expect(bars[1].closest('li')).toHaveStyle({ left: '40.4040404040404%' })
  await userEvent.click(screen.getByRole('button', { name: '이 지점에 자막 추가' }))
  expect(onAddCaption).toHaveBeenCalledWith({ startMs: 6000, endMs: 8000 })
})

it('offers a narration caption its own text, absolute times and removal, and no placement', async () => {
  const plan = clipNarrationFixture().plan
  const change = vi.fn()
  render(
    <ClipTextControls
      plan={plan}
      text={plan.elements!.find((text) => text.instanceId === 'narration-2')!}
      change={change}
      invalid={false}
    />,
  )
  // Absolute output times, not a cut's offsets.
  const start = screen.getByLabelText('자막 시작 (전체 기준)')
  expect(start).toHaveValue(8)
  fireEvent.change(start, { target: { value: '9' } })
  expect(change).toHaveBeenCalledWith(
    { type: 'text', id: 'narration-2', patch: { startMs: 9000, endMs: 12000 } },
    'narration-2:start',
  )
  expect(screen.getByLabelText('자막 원문')).toHaveValue('컷을 건너가는 자막')
  expect(screen.getByRole('button', { name: '문구 삭제' })).toBeInTheDocument()
  // The server places a caption and the project sets its pace and accent.
  for (const label of ['위치', '정렬', '강조색', '자막 속도', '기준']) {
    expect(screen.queryByLabelText(label)).not.toBeInTheDocument()
  }
})
