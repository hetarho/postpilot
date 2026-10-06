import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import { usePrepareClipBrowser } from './usePrepareClipBrowser'
import type { AnalysisPreparationRequest } from '../model/types'

const state = vi.hoisted(() => ({
  signal: undefined as AbortSignal | undefined,
  parent: 'owned',
  gate: undefined as (() => void) | undefined,
}))
vi.mock('@/entities/clip-project', () => ({
  useClipAnalysisPreparationCalls: () => ({}),
  useClipLifecycleApi: () => ({ cancel: vi.fn() }),
}))
vi.mock('../model/prepare', () => ({
  prepareBrowserAnalysis: vi.fn(async (_input, _ports, signal, startParent) => {
    state.signal = signal
    await startParent('preparation')
    await new Promise<void>((resolve) => {
      state.gate = resolve
      signal.addEventListener('abort', () => resolve(), { once: true })
    })
    signal.throwIfAborted()
    return { jobId: state.parent }
  }),
}))
const input = {
  projectId: 'p',
  revision: 1,
  batch: { sources: [] },
} as unknown as AnalysisPreparationRequest
const props = {
  ownerId: 'owner',
  projectId: 'p',
  selectionKey: 'sources',
  job: undefined as GenerationJob | undefined,
  resolveAccess: vi.fn(),
}
afterEach(() => {
  state.signal = undefined
  state.gate = undefined
})
describe('browser preparation page lifecycle', () => {
  it('stops preparation on page leave before late callbacks can publish', async () => {
    const { result } = renderHook(() => usePrepareClipBrowser(props))
    let run!: Promise<unknown>
    await act(async () => {
      run = result.current.preparation.run(input, async () => ({ jobId: 'owned' }))
      await Promise.resolve()
    })
    const assertion = expect(run).rejects.toThrow()
    await act(async () => {
      window.dispatchEvent(new Event('pagehide'))
      await assertion
    })
    expect(state.signal?.aborted).toBe(true)
    expect(result.current.busy).toBe(false)
  })
  it('cancels a changed selection while retaining the same selection through parent ownership', async () => {
    const { result, rerender } = renderHook((p) => usePrepareClipBrowser(p), {
      initialProps: props,
    })
    let run!: Promise<unknown>
    await act(async () => {
      run = result.current.preparation.run(input, async () => ({ jobId: 'owned' }))
      await Promise.resolve()
    })
    rerender({ ...props, job: { id: 'old', status: 'done' } as GenerationJob })
    expect(state.signal?.aborted).toBe(false)
    const assertion = expect(run).rejects.toThrow()
    act(() => {
      rerender({ ...props, selectionKey: 'replacement' })
    })
    await act(async () => {
      await assertion
    })
    expect(state.signal?.aborted).toBe(true)
  })
  it('stops local codecs when its actual durable parent is cancelled', async () => {
    const { result, rerender } = renderHook((p) => usePrepareClipBrowser(p), {
      initialProps: props,
    })
    let run!: Promise<unknown>
    await act(async () => {
      run = result.current.preparation.run(input, async () => ({ jobId: 'owned' }))
      await Promise.resolve()
    })
    const assertion = expect(run).rejects.toThrow()
    act(() => {
      rerender({
        ...props,
        job: { id: 'owned', status: 'running', cancelRequestedAt: 'now' } as GenerationJob,
      })
    })
    await act(async () => {
      await assertion
    })
    expect(state.signal?.aborted).toBe(true)
  })
})
