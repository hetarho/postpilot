import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { MemoryService } from '@/shared/api'
import type { MemoryKind } from '../model/types'
import { invalidateMemories } from './memory-cache'
import { memoryErrorMessage } from './memory-errors'
import { toProtoKind } from './memory-queries'

/** The three write callers live with the entity rather than in the action slices: approving an
 *  extracted candidate needs the create one too, and a feature may not import a sibling feature.
 *  They are plain CRUD over the entity's own cache — no memory write calls a model or enqueues a
 *  job ([I5]) — and none of them touches a post or job cache: a generation froze the TEXTS it
 *  selected at enqueue (MEM-19), so nothing in flight can go stale. */
export function useCreateMemoryCall(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(MemoryService.method.createMemory, {
    onSuccess: () => invalidateMemories(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: memoryErrorMessage(mutation.error),
    /** `sourcePostSlug` is the post a candidate was approved from, and empty for a fact written
     *  by hand. A text the account already holds is not an error there: the server links the
     *  post, advances the memory's last use and answers the existing row (MEM-9). */
    create: (text: string, kind: MemoryKind, tags: string[], sourcePostSlug = '') =>
      mutation.mutateAsync({ text: text.trim(), kind: toProtoKind(kind), tags, sourcePostSlug }),
  }
}

/** Presence is the edit unit: the text saver sends no tags and the facet saver sends no text, so
 *  two edits from two places cannot overwrite each other. The tags go as one patch because the
 *  set is replaced whole — which is also what lets every tag be cleared. */
export function useUpdateMemoryCall(ownerId: string, memoryId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(MemoryService.method.updateMemory, {
    onSuccess: () => invalidateMemories(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: memoryErrorMessage(mutation.error),
    saveText: (text: string) => mutation.mutateAsync({ id: memoryId, text: text.trim() }),
    saveFacets: (kind: MemoryKind, tags: string[]) =>
      mutation.mutateAsync({ id: memoryId, kind: toProtoKind(kind), tags: { tags } }),
  }
}

export function useDeleteMemoryCall(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(MemoryService.method.deleteMemory, {
    onSuccess: () => invalidateMemories(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: memoryErrorMessage(mutation.error),
    remove: (id: string) => mutation.mutateAsync({ id }),
  }
}
