import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ClipGenerationService,
  type ProtoClipUploadText,
  type ProtoClipUploadTextLimits,
} from '@/shared/api'
import { CLIP_CHANNELS, type ClipChannel } from '../config/channel'
import type { ClipQuote } from '../model/types'
import { clipProjectsKey } from './clip-project'
import { toClipQuote } from './credits'

type ModelRef = { providerId: string; modelId: string }

export interface ClipUploadText {
  channel: ClipChannel
  body: string
  tags: string[]
  edited: boolean
  revision: number
  updatedAt: string
}
export interface ClipUploadTextLimits {
  channel: ClipChannel
  maxChars: number
  maxTags: number
  tagsInCaption: boolean
}
function textOf(row: ProtoClipUploadText): ClipUploadText {
  if (!CLIP_CHANNELS.includes(row.channel as ClipChannel) || row.revision <= 0)
    throw new Error('Invalid upload text')
  return { ...row, channel: row.channel as ClipChannel, tags: [...row.tags] }
}
function limitsOf(row: ProtoClipUploadTextLimits): ClipUploadTextLimits {
  if (!CLIP_CHANNELS.includes(row.channel as ClipChannel) || row.maxChars <= 0 || row.maxTags <= 0)
    throw new Error('Invalid upload text limits')
  return { ...row, channel: row.channel as ClipChannel }
}
export function useClipUploadTexts(ownerId: string, projectId: string) {
  const transport = useTransport()
  return useQuery({
    queryKey: [...clipProjectsKey(transport, ownerId), 'upload-text', projectId],
    enabled: !!ownerId && !!projectId,
    staleTime: 0,
    queryFn: async ({ signal }) => {
      const response = await createClient(ClipGenerationService, transport).getClipUploadTexts(
        { projectId },
        { signal },
      )
      return { texts: response.texts.map(textOf), limits: response.limits.map(limitsOf) }
    },
  })
}
export function useClipUploadTextCalls(ownerId: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  return useMemo(() => {
    const client = createClient(ClipGenerationService, transport)
    return {
      keys: [clipProjectsKey(transport, ownerId)],
      async quote(
        input: {
          projectId: string
          channel: ClipChannel
          writeModel: ModelRef
          confirmEdited: boolean
        },
        binding: string,
        signal: AbortSignal,
      ) {
        return toClipQuote(await client.quoteClipUploadText(input, { signal }), binding)
      },
      async start(
        input: {
          projectId: string
          channel: ClipChannel
          writeModel: ModelRef
          confirmEdited: boolean
        },
        quote: ClipQuote,
      ) {
        const response = await client.startClipUploadText({
          ...input,
          quoteId: quote.quoteId,
          approvedMaxCredits: quote.maxCredits,
          cancellationPolicyVersion: quote.cancellationPolicy?.version,
        })
        if (!response.jobId) throw new Error('Missing upload text job')
        await cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
        return response.jobId
      },
      async save(input: {
        projectId: string
        channel: ClipChannel
        body: string
        tags: string[]
        expectedRevision: number
      }) {
        const response = await client.saveClipUploadText(input)
        if (!response.text) throw new Error('Missing saved upload text')
        await cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
        return textOf(response.text)
      },
      async cancel(projectId: string, jobId: string) {
        await client.cancelClipJob({ projectId, jobId })
        await cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
      },
    }
  }, [cache, ownerId, transport])
}
