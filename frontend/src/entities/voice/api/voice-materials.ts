import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import { appFailureFromConnect, ModelRefSchema, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { VoicePrompt, VoiceSampleDetail } from '../model/types'
import { requirePromptPart } from './voice-enums'
import { invalidateVoiceMaterials } from './voice-directory-cache'
import {
  toVoiceSample,
  voicePromptsQueryKey,
  voiceAnalysisQueryKey,
  voiceSampleQueryKey,
} from './voice-queries'

/** The shared prompt set (VOICE-60). It is product copy and never changes within a build. */
export function useVoicePrompts(): {
  prompts: VoicePrompt[]
  isPending: boolean
  isError: boolean
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voicePromptsQueryKey(transport),
    queryFn: () => createClient(VoiceService, transport).listVoicePrompts({}),
    staleTime: Infinity,
  })
  const prompts = useMemo(
    () =>
      query.data?.prompts.map((prompt) => ({
        key: prompt.key,
        part: requirePromptPart(prompt.part),
        photo: prompt.photo,
        text: prompt.text,
      })) ?? [],
    [query.data],
  )
  return { prompts, isPending: query.isPending, isError: query.isError }
}

/** One 학습 글 opened for its owner: the full text and a photo answer's fresh view URL. */
export function useVoiceSample(ownerId: string, voiceId: string, sampleId: string) {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceSampleQueryKey(transport, ownerId, voiceId, sampleId),
    queryFn: () => createClient(VoiceService, transport).getVoiceSample({ voiceId, sampleId }),
    enabled: ownerId !== '' && voiceId !== '' && sampleId !== '',
    // The view URL is presigned for ten minutes (POST-38): an entry older than that is re-read.
    staleTime: 5 * 60 * 1000,
  })
  const detail = useMemo<VoiceSampleDetail | undefined>(
    () =>
      query.data?.sample
        ? {
            sample: toVoiceSample(query.data.sample),
            body: query.data.body,
            photoUrl: query.data.photoUrl,
            photoWidth: query.data.photoWidth,
            photoHeight: query.data.photoHeight,
          }
        : undefined,
    [query.data],
  )
  return {
    detail,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}

/** A photo prompt's upload handshake, as a bare call the answer feature makes around its PUT. */
export function useVoicePhotoUpload(voiceId: string) {
  const transport = useTransport()
  return useMemo(
    () => ({
      create: (promptKey: string) =>
        createClient(VoiceService, transport).createVoicePhotoUpload({ voiceId, promptKey }),
    }),
    [transport, voiceId],
  )
}

export interface VoiceAnswerInput {
  promptKey: string
  body: string
  photo?: { uploadId: string; width: number; height: number }
}

/** Answers one prompt; the answer becomes a 학습 글 and enqueues nothing (VOICE-60). */
export function useAnswerVoicePrompt(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.answerVoicePrompt, {
    onSuccess: () => invalidateVoiceMaterials(queryClient, transport, ownerId, voiceId),
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    ...mutation,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    answer: ({ promptKey, body, photo }: VoiceAnswerInput) =>
      mutation.mutateAsync({
        voiceId,
        promptKey,
        body,
        uploadId: photo?.uploadId ?? '',
        photoWidth: photo?.width ?? 0,
        photoHeight: photo?.height ?? 0,
      }),
  }
}

/** 말투 만들기 and 다시 분석 (VOICE-23): one analyze_voice job on the model the caller names. The
 *  profile carries the active job from then on, so it is re-read. */
export function useAnalyzeVoice(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.analyzeVoice, {
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: voiceAnalysisQueryKey(transport, ownerId, voiceId),
      }),
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    ...mutation,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    analyze: (model: { providerId: string; modelId: string }) =>
      mutation.mutateAsync({ voiceId, model: create(ModelRefSchema, model) }),
  }
}
