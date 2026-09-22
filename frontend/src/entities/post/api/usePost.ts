import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { appFailureFromConnect, type AppFailure, PostService } from '@/shared/api'
import type { PostDraft } from '../model/types'
import { toPostDraft } from './post-queries'

/** Why a post could not be loaded, in the app's terms.
 *
 *  Classified here so the screens never import Connect: a page that has to branch on
 *  `Code.PermissionDenied` has transport knowledge in it, and the next transport change
 *  would then have to visit every screen. */
export type PostLoadFailure = AppFailure

/** `enabled` exists for a consumer that needs the post only in some of its states — a
 *  comparison review asks for the post it ran on only while it still offers to write to it.
 *  Disabled, the hook reports a settled empty read instead of asking for a post nobody is
 *  waiting on. */
export function usePost(
  slug: string,
  { enabled = true }: { enabled?: boolean } = {},
): {
  post: PostDraft | undefined
  isPending: boolean
  isFetching: boolean
  failure: PostLoadFailure | undefined
  refetch: () => void
} {
  // `retry: false` because the two failures that matter here are answers, not transient
  // faults: retrying a 403 or a 404 just asks the same question again, and the editor
  // would sit on a spinner for the length of the retry before saying so.
  const asking = enabled && Boolean(slug)
  const { data, isPending, isFetching, error, refetch } = useQuery(
    PostService.method.getPost,
    { slug },
    {
      enabled: asking,
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
    // A disabled query stays `pending` in react-query's own vocabulary because its data has
    // never arrived. To a caller that deliberately did not ask, that is settled, not
    // loading — reporting it as pending would park the screen on a spinner forever.
    isPending: asking && isPending,
    isFetching,
    failure: error ? appFailureFromConnect(error) : undefined,
    refetch: () => void refetch(),
  }
}
