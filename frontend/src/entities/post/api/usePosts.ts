import { useMemo } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'
import type { PostListItem } from '../model/types'
import { toPostListItem } from './post-queries'

/** The acting user's posts, newest first — the server decides the order (PRD F-8). */
export function usePosts(): {
  posts: PostListItem[]
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const { data, isPending, isError, refetch } = useQuery(PostService.method.listPosts, {})
  const posts = useMemo(() => data?.posts.map(toPostListItem) ?? [], [data])

  return { posts, isPending, isError, refetch: () => void refetch() }
}
