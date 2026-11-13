import { act, renderHook } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { emptyClipProject } from '@/entities/clip-project'
import { clipObservationsFixture } from '@/test/clip-observations'
import { useClipObservationEditing } from './useClipObservationEditing'
import { discardClipObservationQueues } from './observation-queue'

const state = vi.hoisted(() => ({
  save: vi.fn(async (): Promise<void> => undefined),
  invalidate: vi.fn(async () => undefined),
}))
vi.mock('@/entities/clip-project', async (original) => ({
  ...(await original<typeof import('@/entities/clip-project')>()),
  useClipProjectCalls: () => ({ saveObservationEdits: state.save }),
  useClipProjectsKey: () => ['projects'],
}))
vi.mock('@tanstack/react-query', async (original) => ({
  ...(await original<typeof import('@tanstack/react-query')>()),
  useQueryClient: () => ({ invalidateQueries: state.invalidate }),
}))

afterEach(() => {
  discardClipObservationQueues()
  state.save.mockReset().mockResolvedValue(undefined)
  state.invalidate.mockClear()
})
const project = {
  ...emptyClipProject(),
  id: 'observation-draft',
  createdAt: '',
  updatedAt: '',
  editPlanRevision: 0,
  renderedPlanRevision: 0,
}
const segment = { ...clipObservationsFixture().sources[0]!.segments[0]!, speech: 'heard words' }

it('keeps refused speech across another field edit and blocks commits until that speech is corrected', async () => {
  const { result } = renderHook(() => useClipObservationEditing('alice', project, false))
  act(() => result.current.change('a/0', 'speech', segment, ''))
  await expect(result.current.flush(true)).rejects.toMatchObject({ reason: 'CLIP_INVALID_INPUT' })
  act(() => result.current.change('a/0', 'event', segment, 'owner corrected scene'))
  expect(result.current.text('a/0', 'speech', segment)).toBe('')
  expect(result.current.refused('a/0')).toBe('speech')
  expect(result.current.status.failing).toBe(true)
  expect(result.current.hasEdits('a')).toBe(true)
  await expect(result.current.flush(true)).rejects.toMatchObject({ reason: 'CLIP_INVALID_INPUT' })
  expect(state.save).not.toHaveBeenCalled()
  act(() => result.current.change('a/0', 'speech', segment, 'corrected words'))
  await act(() => result.current.flush(true))
  expect(state.save).toHaveBeenCalledExactlyOnceWith(project.id, [
    {
      observationId: 'a/0',
      event: 'owner corrected scene',
      speech: 'corrected words',
    },
  ])
  expect(result.current.refused('a/0')).toBeUndefined()
  expect(result.current.hasEdits('a')).toBe(false)
  expect(result.current.status.failing).toBe(false)
})

it('refuses a new invalid draft made while a committing flush waits for the previous save', async () => {
  let release!: () => void
  state.save.mockImplementationOnce(
    () =>
      new Promise<void>((resolve) => {
        release = resolve
      }),
  )
  const { result } = renderHook(() => useClipObservationEditing('alice', project, false))
  act(() => result.current.change('a/0', 'event', segment, 'corrected event'))
  const flushing = result.current.flush(true)
  // Attach the rejection handler before releasing the flight.
  const refused = expect(flushing).rejects.toMatchObject({ reason: 'CLIP_INVALID_INPUT' })
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
  expect(state.save).toHaveBeenCalledOnce()
  act(() => result.current.change('a/0', 'speech', segment, ''))
  await act(async () => {
    release()
    await refused
  })
  expect(result.current.text('a/0', 'speech', segment)).toBe('')
  expect(result.current.status.failing).toBe(true)
})
