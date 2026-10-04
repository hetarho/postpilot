import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { SpokenVoiceService } from '@/shared/api'
import { API_URL } from '@/shared/config'
import type { SpokenDraftInput } from '../model/types'
import { toSpokenDraft, toSpokenVoice } from './mappers'

export const spokenScope = (transport: Transport, ownerId: string) =>
  ['spoken-voice', transport, ownerId] as const
export function useSpokenLibrary(ownerId: string, includeRemoved = false) {
  const transport = useTransport()
  const client = useMemo(() => createClient(SpokenVoiceService, transport), [transport])
  const voices = useQuery({
    queryKey: [...spokenScope(transport, ownerId), 'voices', includeRemoved],
    queryFn: async () =>
      (await client.listSpokenVoices({ includeRemoved })).voices.map(toSpokenVoice),
    enabled: ownerId !== '',
  })
  const drafts = useQuery({
    queryKey: [...spokenScope(transport, ownerId), 'drafts'],
    queryFn: async () => (await client.listSpokenDrafts({})).drafts.map(toSpokenDraft),
    enabled: ownerId !== '',
  })
  return {
    voices: voices.data ?? [],
    drafts: drafts.data ?? [],
    voicesQuery: voices,
    draftsQuery: drafts,
  }
}
export function useSpokenDraft(ownerId: string, id: string) {
  const transport = useTransport()
  const query = useQuery({
    queryKey: [...spokenScope(transport, ownerId), 'draft', id],
    queryFn: async () =>
      toSpokenDraft(
        (await createClient(SpokenVoiceService, transport).getSpokenDraft({ id })).draft,
      ),
    enabled: ownerId !== '' && id !== '',
  })
  return { ...query, draft: query.data }
}
export function spokenPlaybackUrl(path: string): string {
  // The server issues relative, cookie-authenticated paths, never supplier or bucket URLs.
  if (!/^\/spoken\/audio\/[a-f0-9]{32}$/.test(path)) throw new Error('Invalid spoken sample path')
  return `${import.meta.env.DEV ? '/api' : API_URL.replace(/\/$/, '')}${path}`
}
export function useSpokenActions(ownerId: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  const client = useMemo(() => createClient(SpokenVoiceService, transport), [transport])
  const mutation = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => {
      if (!ownerId) throw new Error('Authentication required')
      return action()
    },
    onSettled: () => cache.invalidateQueries({ queryKey: spokenScope(transport, ownerId) }),
    retry: false,
  })
  async function perform<T extends object>(action: () => Promise<T>): Promise<T> {
    let output: T | undefined
    await mutation.mutateAsync(async () => {
      output = await action()
    })
    if (!output) throw new Error('Missing spoken mutation result')
    return output
  }
  // Keys are caller-owned so a deliberate transport retry can repeat the same request.
  const create = (input: SpokenDraftInput, key: string) =>
    perform(() => client.createSpokenDraft({ input, idempotencyKey: key })).then((r) =>
      toSpokenDraft(r.draft),
    )
  const update = (id: string, revision: bigint, input: SpokenDraftInput, key: string) =>
    perform(() =>
      client.updateSpokenDraft({ id, expectedRevision: revision, input, idempotencyKey: key }),
    ).then((r) => toSpokenDraft(r.draft))
  const removeDraft = (id: string, revision: bigint, key: string) =>
    perform(() => client.deleteSpokenDraft({ id, expectedRevision: revision, idempotencyKey: key }))
  const rename = (id: string, revision: bigint, name: string, key: string) =>
    perform(() =>
      client.renameSpokenVoice({ id, expectedRevision: revision, name, idempotencyKey: key }),
    ).then((r) => toSpokenVoice(r.voice))
  const remove = (id: string, revision: bigint, key: string) =>
    perform(() =>
      client.removeSpokenVoice({ id, expectedRevision: revision, idempotencyKey: key }),
    ).then((r) => toSpokenVoice(r.voice))
  const select = (draftId: string, revision: bigint, candidateId: string, key: string) =>
    perform(() =>
      client.selectSpokenCandidate({
        draftId,
        expectedRevision: revision,
        candidateId,
        idempotencyKey: key,
      }),
    ).then((r) => toSpokenDraft(r.draft))
  const acknowledge = (
    draftId: string,
    revision: bigint,
    candidateId: string,
    playbackId: string,
    key: string,
  ) =>
    perform(() =>
      client.acknowledgeSpokenCandidate({
        draftId,
        expectedRevision: revision,
        candidateId,
        playbackId,
        idempotencyKey: key,
      }),
    ).then((r) => toSpokenDraft(r.draft))
  const sampleAccess = async (id: string) => {
    if (!ownerId) throw new Error('Authentication required')
    const result = await client.getSpokenSampleAccess({ id })
    return {
      playbackId: result.playbackId,
      url: spokenPlaybackUrl(result.url),
      expiresAt: result.expiresAt,
    }
  }
  return {
    create,
    update,
    removeDraft,
    rename,
    remove,
    select,
    acknowledge,
    sampleAccess,
    pending: mutation.isPending,
    error: mutation.error,
  }
}
