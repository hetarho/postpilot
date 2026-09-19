import { useCallback } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient, type QueryKey } from '@tanstack/react-query'
import { getSelectionsQueryKey } from '@/entities/model-catalog/@x/model-experiment'
import { getPostQueryKey, listPostsQueryKey } from '@/entities/post/@x/model-experiment'
import { voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice/@x/model-experiment'

/** What a decided experiment makes stale outside itself. An experiment's outcome is written into
 *  the post it ran on, the account's stage selections and — for an applied analyze winner — the
 *  frozen voice's profile, so the list is stated once here instead of in every review screen
 *  (ARCH-14) and no screen holds a transport to build the keys with (ARCH-17). */
export function useExperimentOwnerRefresh(
  ownerId: string,
  experiment: { postSlug: string; voiceId: string },
): () => Promise<void> {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const { postSlug, voiceId } = experiment
  return useCallback(async () => {
    const keys: QueryKey[] = [listPostsQueryKey(transport), getSelectionsQueryKey(transport)]
    if (postSlug) keys.push(getPostQueryKey(transport, postSlug))
    if (voiceId) {
      keys.push(
        voiceProfileQueryKey(transport, ownerId, voiceId),
        voiceVersionsQueryKey(transport, ownerId, voiceId),
      )
    }
    await Promise.all(keys.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
  }, [ownerId, postSlug, queryClient, transport, voiceId])
}
