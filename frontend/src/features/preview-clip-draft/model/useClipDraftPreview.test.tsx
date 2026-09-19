import { webcrypto } from 'node:crypto'
import type { ReactNode } from 'react'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ClipService, PrepareClipPreviewResponseSchema } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { useClipDraftPreview } from './useClipDraftPreview'

const plan: ClipEditPlan = {
  durationMs: 10000,
  hook: '',
  cuts: [
    {
      id: 'a',
      sourceId: 'a',
      fingerprint: 'a',
      startMs: 0,
      endMs: 10000,
      transitionMs: 0,
      copies: [],
      chips: [],
      volumePermille: 1000,
      playbackRatePermille: 1000,
    },
  ],
  elements: [{ instanceId: 'fixed', kind: 'caption', startMs: 0, endMs: 10000, text: '자막' }],
} as unknown as ClipEditPlan

function harness(
  prepare = vi.fn(async (hash: string) =>
    create(PrepareClipPreviewResponseSchema, {
      draftHash: hash,
      canvasWidth: 1080,
      canvasHeight: 1920,
      nextOffset: -1,
      assets: [
        {
          key: 'glyph',
          instanceId: 'fixed',
          png: new Uint8Array([1]),
          width: 1,
          height: 1,
          startMs: 0,
          endMs: 10000,
        },
      ],
    }),
  ),
) {
  const transport = createRouterTransport((router) =>
    router.service(ClipService, { prepareClipPreview: (req) => prepare(req.draftHash) }),
  )
  const wrapper = ({ children }: { children: ReactNode }) => (
    <TransportProvider transport={transport}>{children}</TransportProvider>
  )
  const view = renderHook(
    (props: { revision: number }) =>
      useClipDraftPreview({ projectId: 'project', revision: props.revision, plan, timeMs: 0 }),
    { wrapper, initialProps: { revision: 1 } },
  )
  return { ...view, prepare }
}

beforeEach(() => {
  vi.stubGlobal('crypto', webcrypto)
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:glyph')
      static revokeObjectURL = vi.fn()
    },
  )
  vi.useFakeTimers({ shouldAdvanceTime: true })
})
afterEach(() => {
  vi.useRealTimers()
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

it('prepares the elements on screen and reports the overlay as ready', async () => {
  const view = harness()
  await waitFor(() => expect(view.prepare).toHaveBeenCalledTimes(1))
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  expect(view.result.current.updating).toBe(false)
  expect(view.result.current.assets).toEqual([
    expect.objectContaining({ instanceId: 'fixed', url: 'blob:glyph' }),
  ])
  expect(view.result.current.failure).toBeUndefined()
})

it('reports a refused preparation as a failure and retries it on demand', async () => {
  const view = harness(
    vi.fn(async () => {
      throw connectAppError('CLIP_PREVIEW_UNAVAILABLE', Code.FailedPrecondition)
    }),
  )
  await waitFor(() => expect(view.result.current.failure?.reason).toBe('CLIP_PREVIEW_UNAVAILABLE'))
  expect(view.result.current.ready).toBe(false)
  act(() => {
    view.result.current.onRetry()
  })
  await waitFor(() => expect(view.prepare).toHaveBeenCalledTimes(2))
})

it('cancels the preparation in flight when the plan it was for is gone', async () => {
  const view = harness()
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  // A newer revision supersedes the answered request: the overlay is no longer the one on
  // screen, so nothing of the old preparation is shown while the new one is asked for.
  view.rerender({ revision: 2 })
  expect(view.result.current.ready).toBe(false)
  await waitFor(() => expect(view.prepare).toHaveBeenCalledTimes(2))
  view.unmount()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:glyph')
})
