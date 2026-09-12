import { act, renderHook } from '@testing-library/react'
import { createRouterTransport, Code, ConnectError } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ClipService, ClipProjectSchema } from '@/shared/api'
import { toClipProject } from '@/entities/clip-project'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useFinalizeClip } from './useFinalizeClip'

function setup(flush: () => Promise<number> = async () => 1) {
  const wire = create(ClipProjectSchema, {
    id: 'clip',
    ratio: 'vertical',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    canFinalize: true,
    result: { id: 'result', contentType: 'video/mp4' },
  })
  let readsFail = false,
    writesFail = false,
    committed = false
  const events: string[] = []
  const transport = createRouterTransport((router) => {
    router.rpc(ClipService.method.getClipProject, () => {
      events.push('read')
      if (readsFail) throw new ConnectError('offline', Code.Unavailable)
      return { project: wire }
    })
    router.rpc(ClipService.method.finalizeClipProject, (input) => {
      events.push('confirm')
      expect(input.expectedRevision).toBe(1)
      expect(input.expectedResultId).toBe('result')
      if (committed)
        Object.assign(wire, {
          finalizedAt: '2026-09-13T00:00:00Z',
          finalizedPlanRevision: 1,
          finalizedResultId: 'result',
          canEdit: false,
          canFinalize: false,
        })
      if (writesFail) {
        readsFail = true
        throw new ConnectError('lost reply', Code.Unavailable)
      }
      return { project: wire }
    })
  })
  const view = renderHook(
    () =>
      useFinalizeClip('owner', toClipProject(wire), async () => {
        events.push('flush')
        return flush()
      }),
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return {
    ...view,
    events,
    wire,
    loseReply: (didCommit: boolean) => {
      writesFail = true
      committed = didCommit
    },
    reconnect: () => {
      readsFail = false
      writesFail = false
    },
  }
}

it('waits for pending settings and correction saves, rejects their failure and suppresses double clicks', async () => {
  let reject!: (error: Error) => void
  const pending = new Promise<number>((_, no) => {
    reject = no
  })
  const view = setup(() => pending)
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
  expect(view.result.current.busy).toBe(false)
  expect(view.result.current.failure).toBeDefined()
})

it.each([false, true])(
  'locks an ambiguous write until an owned read succeeds (committed=%s)',
  async (committed) => {
    const view = setup()
    view.loseReply(committed)
    await act(() => view.result.current.confirm())
    expect(view.result.current.uncertain).toBe(true)
    const count = view.events.length
    await act(() => view.result.current.confirm())
    expect(view.events).toHaveLength(count)
    view.reconnect()
    await act(() => view.result.current.checkAgain())
    expect(view.result.current.busy).toBe(false)
    expect(view.events.filter((e) => e === 'confirm')).toHaveLength(1)
    if (committed) expect(view.result.current.failure).toBeUndefined()
  },
)

it('refuses a new plan revision read after flushing instead of confirming another tab’s draft', async () => {
  const view = setup()
  view.wire.editPlanRevision = 2
  await act(() => view.result.current.confirm())
  expect(view.events).toEqual(['flush', 'read'])
  expect(view.result.current.failure?.reason).toBe('CLIP_FINALIZATION_CONFLICT')
})
