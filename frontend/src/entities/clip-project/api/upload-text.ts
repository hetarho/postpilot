import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ClipGenerationService,
  type ProtoClipUploadText,
  type ProtoClipUploadTextLimits,
} from '@/shared/api'
import type { ClipQuote } from '../model/types'
import { clipProjectsKey } from './clip-project'
import { toClipQuote } from './credits'

type ModelRef = { providerId: string; modelId: string }

export interface ClipUploadText {
  body: string
  tags: string[]
  edited: boolean
  revision: number
  updatedAt: string
}
export interface ClipUploadTextLimits {
  maxChars: number
  maxTags: number
  tagsInCaption: boolean
}
// The proto still carries a channel until T867 removes it; a project has one upload text (CLIP-209),
// so the first row is it and no channel is read.
function textOf(row: ProtoClipUploadText): ClipUploadText {
  if (row.revision <= 0) throw new Error('Invalid upload text')
  return {
    body: row.body,
    tags: [...row.tags],
    edited: row.edited,
    revision: row.revision,
    updatedAt: row.updatedAt,
  }
}
function limitsOf(row: ProtoClipUploadTextLimits): ClipUploadTextLimits {
  if (row.maxChars <= 0 || row.maxTags <= 0) throw new Error('Invalid upload text limits')
  return { maxChars: row.maxChars, maxTags: row.maxTags, tagsInCaption: row.tagsInCaption }
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
      const text = response.texts[0]
      const limits = response.limits[0]
      return { text: text && textOf(text), limits: limits && limitsOf(limits) }
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
