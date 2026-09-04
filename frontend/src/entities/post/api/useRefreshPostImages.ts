import { useCallback } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { getPostQueryKey } from './post-queries'

/** Asks the server for the post again, for its PHOTO URLS.
 *
 *  A view URL is presigned and short-lived (the API's `presignGetTTL`), and nothing else on the
 *  editor refetches while it sits open: the post query is refetched on mount, window-focus
 *  refetching is off, and a draft save deliberately patches the cached image list in place rather
 *  than invalidating it. So a panel left open past the TTL holds URLs that no longer resolve, and
 *  the next photo to be loaded from one — a lazy image scrolled to, or a preview remounted by a
 *  tab switch — never paints.
 *
 *  It is a callback rather than a poll on purpose. Once a photo has painted it is copied from its
 *  own pixels and never re-requested, so refreshing on a timer would re-download every photo on
 *  the panel every few minutes to fix a URL that nothing is going to use. The caller asks when a
 *  photo actually fails to load, and when it opens a surface that is about to load them. */
export function useRefreshPostImages(slug: string): () => void {
  const queryClient = useQueryClient()
  const transport = useTransport()
  return useCallback(() => {
    if (!slug) return
    void queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, slug) })
  }, [queryClient, slug, transport])
}
