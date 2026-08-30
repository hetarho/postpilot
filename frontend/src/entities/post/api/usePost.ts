import { useMemo } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import { useQuery } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'
import type { PostDraft } from '../model/types'
import { toPostDraft } from './post-queries'

/** Why a post could not be loaded, in the app's terms.
 *
 *  Classified here so the screens never import Connect: a page that has to branch on
 *  `Code.PermissionDenied` has transport knowledge in it, and the next transport change
 *  would then have to visit every screen. */
export type PostLoadFailure = 'forbidden' | 'not-found' | 'unavailable'

function classify(error: unknown): PostLoadFailure {
  switch (ConnectError.from(error).code) {
    // A slug that exists but is someone else's is 403, not 404 (spec/policy/posts.md).
    case Code.PermissionDenied:
      return 'forbidden'
    case Code.NotFound:
      return 'not-found'
    default:
      return 'unavailable'
  }
}

export function usePost(slug: string): {
  post: PostDraft | undefined
  isPending: boolean
  isFetching: boolean
  failure: PostLoadFailure | undefined
  refetch: () => void
} {
  // `retry: false` because the two failures that matter here are answers, not transient
  // faults: retrying a 403 or a 404 just asks the same question again, and the editor
  // would sit on a spinner for the length of the retry before saying so.
  const { data, isPending, isFetching, error, refetch } = useQuery(
    PostService.method.getPost,
    { slug },
    {
      retry: false,
      // A cached post may still be fresh while its image capabilities are not. Draft
      // saves deliberately preserve the cached image list, so they also keep its
      // short-lived presigned URLs (or the blob URL handed off after an upload) while
      // advancing React Query's dataUpdatedAt. Always ask GetPost for fresh URLs when
      // the editor is entered; cached data still paints the first render immediately.
      refetchOnMount: 'always',
    },
  )

  // Memoised because the editor treats the post object as the identity of one editing
  // session: a fresh object on every render would tear the autosave queue down and
  // re-attach it on each keystroke. react-query's structural sharing keeps `data` stable
  // while the server's answer has not changed, so this is stable too.
  const post = useMemo(() => (data?.post ? toPostDraft(data.post) : undefined), [data])

  return {
    post,
    isPending,
    isFetching,
    failure: error ? classify(error) : undefined,
    refetch: () => void refetch(),
  }
}
