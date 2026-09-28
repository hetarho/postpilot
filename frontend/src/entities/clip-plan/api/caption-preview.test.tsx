import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  ClipPlanService,
  GetClipCaptionStyleSamplesResponseSchema,
  GetClipRegionPresetSamplesResponseSchema,
  type AppFailureReason,
} from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useClipCaptionStyleSamples, useClipRegionPresetSamples } from './caption-preview'

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
