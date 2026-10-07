import { createClient } from '@connectrpc/connect'
import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import { ModelRefSchema, VoiceService } from '@/shared/api'
import type { VoiceAnalysisEstimate, VoiceMaterialUpdate } from '../model/types'
import { toVoiceSample } from './voice-queries'
import { invalidateVoiceMaterials } from './voice-directory-cache'

/** Owner/voice-scoped source edits perform no provider work. */
export function useVoiceMaterialEditor(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queries = useQueryClient()
  return useMemo(
    () => ({
      update: async (input: VoiceMaterialUpdate, signal?: AbortSignal) => {
        if (
          !ownerId ||
          !voiceId ||
          !input.sampleId ||
          input.expectedContentRevision <= 0n ||
          !input.operationKey
        )
          throw new Error('An owned material revision is required')
        const result = await createClient(VoiceService, transport).updateVoiceSample(
          {
            voiceId,
            sampleId: input.sampleId,
            expectedContentRevision: input.expectedContentRevision,
            operationKey: input.operationKey,
            label: input.label,
            body: input.body,
            photoUploadId: input.photo?.uploadId,
            photoWidth: input.photo?.width,
            photoHeight: input.photo?.height,
          },
          { signal },
        )
        if (
          !result.sample ||
          result.sample.id !== input.sampleId ||
          result.sample.contentRevision < input.expectedContentRevision
        )
          throw new Error('The material update was not confirmed')
        const sample = toVoiceSample(result.sample)
        invalidateVoiceMaterials(queries, transport, ownerId, voiceId)
        return sample
      },
    }),
    [ownerId, voiceId, transport, queries],
  )
}

/** Advisory customer-credit estimate from the same server snapshot rules as analysis admission. */
export function useVoiceAnalysisEstimator(ownerId: string, voiceId: string) {
  const transport = useTransport()
  return useMemo(
    () => ({
      estimate: async (
        model: { providerId: string; modelId: string },
        signal?: AbortSignal,
      ): Promise<VoiceAnalysisEstimate> => {
        if (!ownerId || !voiceId) throw new Error('An owned writing voice is required')
        const result = await createClient(VoiceService, transport).estimateVoiceAnalysis(
          { voiceId, model: create(ModelRefSchema, model) },
          { signal },
        )
        if (result.free) return { free: true, credits: 0 }
        if (
          result.credits === undefined ||
          !Number.isSafeInteger(result.credits) ||
          result.credits < 0
        )
          throw new Error('No bounded analysis estimate was returned')
        return { free: false, credits: Number(result.credits) }
      },
    }),
    [ownerId, voiceId, transport],
  )
}
