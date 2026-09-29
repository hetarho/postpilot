import { webcrypto } from 'node:crypto'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipRenderService } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { clipPreviewRequest } from './preview'

afterEach(() => vi.unstubAllGlobals())

it('refuses an oversized Connect JSON body before sending or truncating the current draft', async () => {
  vi.stubGlobal('crypto', webcrypto)
  const send = vi.fn(() => {
    throw new Error('oversized request reached transport')
  })
  const transport = createRouterTransport((router) =>
    router.service(ClipRenderService, { prepareClipPreview: send }),
  )
  const plan: ClipEditPlan = {
    nativeComposition: true,
    durationMs: 15000,
    cuts: [],
    elements: Array.from({ length: 800 }, (_, i) => ({
      instanceId: `copy-${i}`,
      elementId: 'caption',
      cutId: 'a',
      kind: 'fixed',
      role: 'caption',
      text: '한'.repeat(60),
      rows: [],
      style: 'bold',
      position: 'bottom',
      align: 'center',
      basis: 'cut',
      pace: 'steady',
      accent: '',
      keyword: '',
      resolvedStartMs: 0,
      resolvedEndMs: 15000,
      groupId: '',
      itemId: '',
    })),
  }
  const before = JSON.stringify(plan)
  // Its binary plan fits; repeated JSON field names and UTF-8 text do not.
  const request = await clipPreviewRequest(transport, 'owned', 1, plan)
  await expect(request.load(['copy-0'], 0, new AbortController().signal)).rejects.toThrow(
    'CLIP_PREVIEW_TOO_LARGE',
  )
  expect(send).not.toHaveBeenCalled()
  expect(JSON.stringify(plan)).toBe(before)
})

const smallPlan: ClipEditPlan = {
  nativeComposition: true,
  durationMs: 15000,
  cuts: [],
  elements: [],
}

// The page's own preview can hold the owner's preview lock while a browser render asks for
// its caption frames; the run is asked for again rather than failing the render.
it('asks again for a caption frame run the preview lock refused as busy', async () => {
  vi.stubGlobal('crypto', webcrypto)
  let calls = 0
  const transport = createRouterTransport((router) =>
    router.service(ClipRenderService, {
      prepareClipCaptionFrames: () => {
        if (++calls === 1) throw connectAppError('CLIP_PREVIEW_BUSY', Code.ResourceExhausted)
        return { cells: 3, cellWidth: 10, cellHeight: 10, columns: 3, nextOffset: -1 }
      },
    }),
  )
  const request = await clipPreviewRequest(transport, 'owned', 1, smallPlan)
  const page = await request.frames('caption', 0, new AbortController().signal)
  expect(page.cells).toBe(3)
  expect(calls).toBe(2)
})

it('keeps any other caption frame refusal final', async () => {
  vi.stubGlobal('crypto', webcrypto)
  let calls = 0
  const transport = createRouterTransport((router) =>
    router.service(ClipRenderService, {
      prepareClipCaptionFrames: () => {
        calls++
        throw connectAppError('CLIP_PREVIEW_TIMEOUT', Code.DeadlineExceeded)
      },
    }),
  )
  const request = await clipPreviewRequest(transport, 'owned', 1, smallPlan)
  await expect(request.frames('caption', 0, new AbortController().signal)).rejects.toThrow()
  expect(calls).toBe(1)
})

// CLIP-192: a browser render's assets and frames are asked for by the render's name, so the
// server draws them on the grounds it sampled for it; the editing preview never names one.
it('names the browser render on both of its requests, and nothing else ever does', async () => {
  vi.stubGlobal('crypto', webcrypto)
  const seen: string[] = []
  const transport = createRouterTransport((router) =>
    router.service(ClipRenderService, {
      prepareClipPreview: (request) => {
        seen.push(`assets:${request.renderId}`)
        return { draftHash: request.draftHash, nextOffset: -1 }
      },
      prepareClipCaptionFrames: (request) => {
        seen.push(`frames:${request.renderId}`)
        return { cells: 1, cellWidth: 10, cellHeight: 10, columns: 1, nextOffset: -1 }
      },
    }),
  )
  const signal = new AbortController().signal
  const render = await clipPreviewRequest(transport, 'owned', 1, smallPlan, 'render')
  await render.load([], 0, signal)
  await render.frames('caption', 0, signal)
  const editing = await clipPreviewRequest(transport, 'owned', 1, smallPlan)
  await editing.load([], 0, signal)
  await editing.frames('caption', 0, signal)
  expect(seen).toEqual(['assets:render', 'frames:render', 'assets:', 'frames:'])
})
