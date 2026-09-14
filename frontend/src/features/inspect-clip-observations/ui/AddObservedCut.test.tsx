import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { clipObservationsFixture } from '@/test/clip-observations'
import { clipTimelineFixture } from '@/test/clip-editing'
import { AddObservedCut } from './AddObservedCut'

it('selects an unused observed range by precise source milliseconds without changing the evidence', async () => {
  const observation = clipObservationsFixture().sources[0]
  const segment = {
    ...observation.segments[0],
    startMs: 8000,
    endMs: 15000,
    focal: { x: 0.2, y: 0.6 },
  }
  const before = structuredClone(segment)
  const onAddCut = vi.fn(async () => {})
  render(
    <AddObservedCut
      observation={observation}
      segment={segment}
      plan={clipTimelineFixture().plan}
      onAddCut={onAddCut}
    />,
  )
  const add = screen.getByRole('button', { name: '이 구간을 컷으로 추가' })
  expect(add).toBeDisabled()
  fireEvent.change(screen.getByRole('spinbutton', { name: '추가할 원본 시작 (초)' }), {
    target: { value: '10.123' },
  })
  expect(add).toBeEnabled()
  await userEvent.click(add)
  expect(onAddCut).toHaveBeenCalledWith({
    source: observation.source,
    segment,
    startMs: 10123,
    endMs: 15000,
  })
  expect(segment).toEqual(before)
})
