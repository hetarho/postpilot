import { useCallback } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { getSelectionsQueryKey } from '@/entities/model-catalog/@x/model-experiment'
import { invalidateWrittenPost } from '@/entities/post/@x/model-experiment'

/** What a decided experiment makes stale outside itself. An experiment's outcome is written into
 *  the post it ran on and the account's stage selections, so the list is stated once here instead
 *  of in every review screen (ARCH-14) and no screen holds a transport to build the keys with
 *  (ARCH-17). */
export function useExperimentOwnerRefresh(experiment: { postSlug: string }): () => Promise<void> {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const { postSlug } = experiment
  return useCallback(async () => {
    await Promise.all([
      invalidateWrittenPost(queryClient, transport, postSlug),
      queryClient.invalidateQueries({ queryKey: getSelectionsQueryKey(transport) }),
    ])
  }, [postSlug, queryClient, transport])
}
