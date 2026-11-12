import { act, renderHook } from '@testing-library/react'
import { createRouterTransport, Code, ConnectError } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { expect, it, vi } from 'vitest'
import { ClipGenerationService, ClipProjectSchema } from '@/shared/api'
import { toClipProject } from '@/entities/clip-project'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useConfirmClip } from './useConfirmClip'

function setup(flush: () => Promise<number>, refuse = false) {
  const wire = create(ClipProjectSchema, { id: 'clip', ratio: 'vertical', editPlanRevision: 1 })
  const events: string[] = []
  const onConfirmed = vi.fn()
  const transport = createRouterTransport((router) => {
    router.rpc(ClipGenerationService.method.confirmClipProject, (input) => {
      events.push(`confirm:${input.expectedPlanRevision}`)
      if (refuse) throw new ConnectError('stale', Code.Aborted)
      wire.editPlanRevision = input.expectedPlanRevision
      wire.confirmedPlanRevision = input.expectedPlanRevision
      return { project: wire }
    })
  })
  const view = renderHook(
    () =>
      useConfirmClip({
        ownerId: 'owner',
        project: toClipProject(wire),
        flush: async () => {
          events.push('flush')
          return flush()
        },
        onConfirmed,
      }),
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { ...view, events, onConfirmed }
}

it('flushes first, confirms the revision the flush saved and hands the project on', async () => {
  const view = setup(async () => 3)
  await act(() => view.result.current.confirm())
  expect(view.events).toEqual(['flush', 'confirm:3'])
  expect(view.onConfirmed).toHaveBeenCalledWith(
    expect.objectContaining({ confirmed: expect.objectContaining({ planRevision: 3 }) }),
  )
  expect(view.result.current.failure).toBeUndefined()
})

it('never confirms after a failed save and suppresses a double press', async () => {
  let reject!: (error: Error) => void
  const view = setup(
    () =>
      new Promise<number>((_, no) => {
        reject = no
      }),
  )
  let first!: Promise<void>
  act(() => {
    first = view.result.current.confirm()
    void view.result.current.confirm()
  })
  expect(view.events).toEqual(['flush'])
  await act(async () => {
    reject(new Error('save failed'))
    await first
  })
  expect(view.events).toEqual(['flush'])
  expect(view.onConfirmed).not.toHaveBeenCalled()
  expect(view.result.current.failure).toBeDefined()
  expect(view.result.current.pending).toBe(false)
})

it('keeps the step and states a refused confirmation', async () => {
  const view = setup(async () => 1, true)
  await act(() => view.result.current.confirm())
  expect(view.events).toEqual(['flush', 'confirm:1'])
  expect(view.onConfirmed).not.toHaveBeenCalled()
  expect(view.result.current.failure).toBeDefined()
})
