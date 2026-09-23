import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { experimentListQueriesKey } from '@/entities/model-experiment/@x/post'
import { invalidateQuality } from '@/entities/quality/@x/post'
import { appFailureFromConnect, PostService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

/** Deletes one owned post. The server delete is hard and unrecoverable, so this hook has
 *  no optimistic path: nothing is dropped from a cache until the RPC has succeeded. */
export function useDeletePost() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.deletePost, {
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
      // Removed, not invalidated: refetching a slug that is now a 404 would replace a
      // clean cache miss with a stored error the next visitor of that key would read.
      if (variables.slug) {
        queryClient.removeQueries({ queryKey: getPostQueryKey(transport, variables.slug) })
      }
      // The post's experiments survive with a null post_slug, so their list is stale.
      void queryClient.invalidateQueries({ queryKey: experimentListQueriesKey(transport) })
      // A deleted published post leaves the window every other post's M2 and the aggregate read
      // (POST-85).
      void invalidateQuality(queryClient, transport)
    },
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    ...mutation,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    remove: (slug: string) => mutation.mutateAsync({ slug }),
  }
}
