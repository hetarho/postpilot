import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useCountUp } from './useCountUp'

/** A frame scheduler under test control: `step` runs every pending callback at the given clock. */
function fakeFrames() {
  let queue: FrameRequestCallback[] = []
  let now = 0
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    queue.push(callback)
    return queue.length
  })
  vi.stubGlobal('cancelAnimationFrame', () => {
    queue = []
  })
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  return {
    step(to: number) {
      now = to
      const pending = queue
      queue = []
      act(() => {
        for (const callback of pending) callback(now)
      })
    },
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('useCountUp', () => {
  // The first figure is shown at once: a ladder must never open at zero and climb.
  it('shows the first value immediately', () => {
    fakeFrames()
    const { result } = renderHook(() => useCountUp(52, 400))
    expect(result.current).toBe(52)
  })

  it('counts through intermediate values to a new target and settles exactly on it', () => {
    const frames = fakeFrames()
    const { result, rerender } = renderHook(({ value }) => useCountUp(value, 400), {
      initialProps: { value: 10 },
    })
    rerender({ value: 110 })
    expect(result.current).toBe(10)

    frames.step(100)
    const partial = result.current
    expect(partial).toBeGreaterThan(10)
    expect(partial).toBeLessThan(110)

    frames.step(400)
    expect(result.current).toBe(110)
  })

  // A slider dragged faster than one count finishes restarts from the figure on screen, so the
  // number never jumps backwards to a stale start.
  it('restarts a retargeted count from the figure currently shown', () => {
    const frames = fakeFrames()
    const { result, rerender } = renderHook(({ value }) => useCountUp(value, 400), {
      initialProps: { value: 0 },
    })
    rerender({ value: 100 })
    frames.step(200)
    const midway = result.current
    expect(midway).toBeGreaterThan(0)

    rerender({ value: 20 })
    frames.step(200)
    expect(result.current).toBe(midway)
    frames.step(600)
    expect(result.current).toBe(20)
  })

  // Where the browser asks for reduced motion the count has no duration: the first frame is
  // already the destination, with no intermediate figure ever shown.
  it('jumps straight to the target under reduced motion', () => {
    const frames = fakeFrames()
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const { result, rerender } = renderHook(({ value }) => useCountUp(value, 400), {
      initialProps: { value: 1 },
    })
    rerender({ value: 99 })
    frames.step(1)
    expect(result.current).toBe(99)
  })
})
