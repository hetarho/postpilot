import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { AnyRouter } from '@tanstack/react-router'
import { IDLE_PRELOAD_PATHS, preloadWhenIdle } from './idle-preload'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

function fakeRouter() {
  const preloadRoute = vi.fn<(options: { to: string }) => Promise<undefined>>(() =>
    Promise.resolve(undefined),
  )
  return { router: { preloadRoute } as unknown as AnyRouter, preloadRoute }
}

it('preloads the next screens one at a time once the screen in view has started', async () => {
  const { router, preloadRoute } = fakeRouter()
  preloadWhenIdle(router, '/')
  await vi.advanceTimersByTimeAsync(500)
  expect(preloadRoute).not.toHaveBeenCalled()
  await vi.runAllTimersAsync()
  expect(preloadRoute.mock.calls.map(([options]) => options.to)).toEqual(
    IDLE_PRELOAD_PATHS.filter((path) => path !== '/'),
  )
  expect(IDLE_PRELOAD_PATHS).toEqual(
    expect.arrayContaining(['/posts', '/clips', '/ai-models', '/voices', '/templates']),
  )
})

it('stops where it is when the shell leaves, and fetches nothing on a data saver', async () => {
  const { router, preloadRoute } = fakeRouter()
  const stop = preloadWhenIdle(router, '/')
  stop()
  await vi.runAllTimersAsync()
  expect(preloadRoute).not.toHaveBeenCalled()

  vi.stubGlobal('navigator', { ...navigator, connection: { saveData: true } })
  try {
    preloadWhenIdle(router, '/')
    await vi.runAllTimersAsync()
    expect(preloadRoute).not.toHaveBeenCalled()
  } finally {
    vi.unstubAllGlobals()
  }
})
