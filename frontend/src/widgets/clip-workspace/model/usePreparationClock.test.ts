import { act, renderHook } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { usePreparationClock } from './usePreparationClock'

afterEach(() => vi.useRealTimers())

it('updates at each retained-source deadline and on resume without polling or starting work', () => {
  vi.useFakeTimers()
  const start = Date.parse('2026-11-11T00:00:00Z')
  vi.setSystemTime(start)
  const deadlines = [new Date(start + 1000).toISOString(), new Date(start + 2000).toISOString()]
  const view = renderHook(() => usePreparationClock(deadlines))
  expect(view.result.current).toBe(start)
  act(() => vi.advanceTimersByTime(1000))
  expect(view.result.current).toBe(start + 1000)
  act(() => vi.advanceTimersByTime(1000))
  expect(view.result.current).toBe(start + 2000)
  act(() => {
    vi.setSystemTime(start + 5000)
    window.dispatchEvent(new Event('focus'))
  })
  expect(view.result.current).toBe(start + 5000)
  view.unmount()
  expect(vi.getTimerCount()).toBe(0)
})
