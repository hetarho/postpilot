import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import type { PostListItem } from '../model/types'
import { toPostListItem } from './post-queries'

/** The acting user's posts, newest first — the server decides the order (PRD F-8). */
export function usePosts(): {
  posts: PostListItem[]
  isPending: boolean
  isError: boolean
  /** True while a fetch is in flight, including a retry of a query that already failed.
   *  react-query keeps `status: 'error'` across such a refetch, so `isPending` never flips back —
   *  this is the only signal a retry control has to show that it did something. */
  isFetching: boolean
  refetch: () => void
} {
  const { data, isPending, isFetching, isError, refetch } = useQuery(
    PostService.method.listPosts,
    {},
    {
      refetchInterval: (state) =>
        state.state.data?.posts.some((post) => post.activeJob) ? POLL_INTERVAL_MS : false,
    },
  )
  const posts = useMemo(() => data?.posts.map(toPostListItem) ?? [], [data])

  return { posts, isPending, isFetching, isError, refetch: () => void refetch() }
}
