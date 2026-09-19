import { expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import type { GenerationJob } from '@/entities/generation-job'
import {
  useDiscardQueueWhenFinalized,
  useSoundBatchHandoff,
  useUnsavedNotice,
  useUploadAttemptLifecycle,
} from './lifecycle'

const job = (patch: Partial<GenerationJob>) =>
  ({ id: 'job', kind: 'generate_clip', status: 'running', ...patch }) as GenerationJob

it('closes the upload attempt on the job’s outcome, whatever step the owner is on', () => {
  const upload = { finishAttempt: vi.fn() }
  const view = renderHook(({ j }: { j?: GenerationJob }) => useUploadAttemptLifecycle(j, upload), {
    initialProps: { j: undefined as GenerationJob | undefined },
  })
  expect(upload.finishAttempt).not.toHaveBeenCalled()
  view.rerender({ j: job({ status: 'running' }) })
  expect(upload.finishAttempt).not.toHaveBeenCalled()
  for (const status of ['done', 'failed', 'cancelled'] as const) {
    upload.finishAttempt.mockClear()
    view.rerender({ j: job({ id: status, status }) })
    expect(upload.finishAttempt).toHaveBeenCalledWith(status, status)
  }
})

it('hands a saved sound batch back to the upload session exactly once', () => {
  const accept = vi.fn()
  const batch = { id: 'batch' }
  const view = renderHook(({ b }: { b?: { id: string } }) => useSoundBatchHandoff(b, accept), {
    initialProps: { b: undefined as { id: string } | undefined },
  })
  expect(accept).not.toHaveBeenCalled()
  view.rerender({ b: batch })
  view.rerender({ b: batch })
  expect(accept).toHaveBeenCalledTimes(1)
  expect(accept).toHaveBeenCalledWith(batch)
})

it('drops the save queue when the project is finalized, and not before', () => {
  const discard = vi.fn()
  const view = renderHook(
    ({ finalized }: { finalized?: object }) =>
      useDiscardQueueWhenFinalized('clip', finalized, discard),
    { initialProps: { finalized: undefined as object | undefined } },
  )
  expect(discard).not.toHaveBeenCalled()
  view.rerender({ finalized: { at: '2026-09-20T00:00:00Z' } })
  expect(discard).toHaveBeenCalledWith('clip')
})

it('tells the page when work is held, and releases it on unmount', () => {
  const report = vi.fn()
  const view = renderHook(({ unsaved }) => useUnsavedNotice(unsaved, report), {
    initialProps: { unsaved: false },
  })
  expect(report).toHaveBeenLastCalledWith(false)
  view.rerender({ unsaved: true })
  expect(report).toHaveBeenLastCalledWith(true)
  view.unmount()
  expect(report).toHaveBeenLastCalledWith(false)
})
