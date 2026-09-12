import { webcrypto } from 'node:crypto'
import { createRouterTransport } from '@connectrpc/connect'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipService } from '@/shared/api'
import type { ClipEditPlan } from '../model/edit-plan'
import { clipPreviewRequest } from './preview'

afterEach(() => vi.unstubAllGlobals())

it('refuses an oversized Connect JSON body before sending or truncating the current draft', async () => {
  vi.stubGlobal('crypto', webcrypto)
  const send = vi.fn(() => {
    throw new Error('oversized request reached transport')
  })
  const transport = createRouterTransport((router) =>
    router.service(ClipService, { prepareClipPreview: send }),
  )
  const plan: ClipEditPlan = {
    nativeComposition: true,
    durationMs: 15000,
    hook: '',
    cuts: [],
    elements: Array.from({ length: 800 }, (_, i) => ({
      instanceId: `copy-${i}`,
      elementId: 'caption',
      cutId: 'a',
      kind: 'fixed',
      role: 'caption',
      text: '한'.repeat(60),
      rows: [],
      style: 'clean',
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
