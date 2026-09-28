import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  ClipPlanService,
  GetClipCaptionPreviewResponseSchema,
  GetClipCaptionStyleSamplesResponseSchema,
  GetClipRegionPresetSamplesResponseSchema,
  type AppFailureReason,
} from '@/shared/api'
import { clipTimelineFixture } from '@/test/clip-editing'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import {
  captionDrawingKey,
  useClipCaptionPreview,
  useClipCaptionStyleSamples,
  useClipRegionPresetSamples,
} from './caption-preview'

/** A server that refuses the first `refusals` calls of each sample RPC with `reason`, the way
 *  the owner's preview lock refuses the second of ①'s two simultaneous sample requests. */
function samplesServer(refusals: number, reason: AppFailureReason) {
  const calls = { captions: 0, regions: 0 }
  const refuse = (n: number) => {
    if (n <= refusals) throw connectAppError(reason, Code.ResourceExhausted)
  }
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipPlanService.method.getClipCaptionStyleSamples, () => {
      refuse(++calls.captions)
      return create(GetClipCaptionStyleSamplesResponseSchema, {
        ratio: 'vertical',
        samples: [{ instanceId: 'bold', style: 'bold', svg: '<g/>' }],
      })
    })
    rpc(ClipPlanService.method.getClipRegionPresetSamples, () => {
      refuse(++calls.regions)
      return create(GetClipRegionPresetSamplesResponseSchema, {
        ratio: 'vertical',
        intro: [{ preset: 'a', svg: '<g/>' }],
      })
    })
  })
  return { calls, wrapper: withProviders(transport, createTestQueryClient()) }
}

it('asks again for samples the preview lock refused as busy', async () => {
  const { calls, wrapper } = samplesServer(1, 'CLIP_PREVIEW_BUSY')
  const regions = renderHook(() => useClipRegionPresetSamples('project', '슬롯 {n}'), { wrapper })
  const captions = renderHook(() => useClipCaptionStyleSamples('project', true), { wrapper })
  await waitFor(() => expect(regions.result.current.data?.intro[0]?.preset).toBe('a'))
  await waitFor(() => expect(captions.result.current.data?.captions[0]?.style).toBe('bold'))
  expect(calls).toEqual({ captions: 2, regions: 2 })
})

it('keeps any other refusal final, so ① shows the names alone', async () => {
  const { calls, wrapper } = samplesServer(1, 'CLIP_PREVIEW_UNAVAILABLE')
  const regions = renderHook(() => useClipRegionPresetSamples('project', '슬롯 {n}'), { wrapper })
  await waitFor(() => expect(regions.result.current.isError).toBe(true))
  expect(calls.regions).toBe(1)
})

// CLIP-143, CDS-83: the caption sheet draws what the server draws for the plan as it stands. The
// owner's style — one outside the AI set included — reaches the server, and an answer for an
// earlier style that arrives late never lands on the caption as it is now.
it('draws the current style, never an earlier answer that arrives late', async () => {
  const asked: string[] = []
  let release = () => {}
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipPlanService.method.getClipCaptionPreview, async (req) => {
      const style = req.plan?.elements?.[0]?.ownerStyle ?? ''
      asked.push(style)
      if (style === 'neon') await new Promise<void>((resolve) => (release = resolve))
      return create(GetClipCaptionPreviewResponseSchema, {
        canvas: { width: 1080, height: 1920 },
        safeArea: { x: 64, y: 40, width: 952, height: 1380 },
        captions: [{ instanceId: 'caption-a', style, svg: `<g data-style="${style}"/>` }],
      })
    })
  })
  const styled = (ownerStyle: string) => {
    const plan = clipTimelineFixture().plan
    return {
      ...plan,
      elements: plan.elements!.map((text, i) => (i === 0 ? { ...text, ownerStyle } : text)),
    }
  }
  const view = renderHook(({ plan }) => useClipCaptionPreview('project', 1, plan, true), {
    wrapper: withProviders(transport, createTestQueryClient()),
    initialProps: { plan: styled('neon') },
  })
  await waitFor(() => expect(asked).toEqual(['neon']))
  view.rerender({ plan: styled('film') })
  await waitFor(() => expect(view.result.current.data?.captions[0]?.style).toBe('film'))
  release()
  await new Promise((resolve) => setTimeout(resolve, 20))
  expect(view.result.current.data?.captions[0]?.style).toBe('film')
  expect(asked).toEqual(['neon', 'film'])
})

// Only what a caption DRAWS asks the server again: its style and its size do, its place does not.
it('keys the drawing by style and size, not by place', () => {
  const plan = clipTimelineFixture().plan
  const edit = (patch: Partial<NonNullable<typeof plan.elements>[number]>) => ({
    ...plan,
    elements: plan.elements!.map((text, i) => (i === 0 ? { ...text, ...patch } : text)),
  })
  const key = captionDrawingKey(plan)
  expect(captionDrawingKey(edit({ ownerStyle: 'neon' }))).not.toBe(key)
  expect(captionDrawingKey(edit({ ownerSizePx: 70 }))).not.toBe(key)
  expect(captionDrawingKey(edit({ ownerPosition: { x: 10, y: 20 } }))).toBe(key)
})
