import { useMemo } from 'react'
import { keepPreviousData } from '@tanstack/react-query'
import { useInfiniteQuery } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'
import { POLL_INTERVAL_MS, POSTS_LIST_GC_MS, POSTS_PAGE_SIZE } from '@/shared/config'
import type { PostListItem, PostStatus } from '../model/types'
import { toPostListItem } from './post-queries'

/** What the list is narrowed by. The server applies both over every post the account owns, not
 *  over the rows loaded so far (POST-91), so the query is sent as typed, already settled. */
export interface PostListNarrowing {
  q?: string
  status?: PostStatus
}

/** The acting user's posts, newest first, a page at a time (POST-90). Each narrowing is its own
 *  cache entry whose pages outlive the screen for `POSTS_LIST_GC_MS`, so coming back from a post
 *  renders the rows that were loaded (POST-93). */
export function usePostList(narrowing: PostListNarrowing): {
  posts: PostListItem[]
  isPending: boolean
  isError: boolean
  isFetching: boolean
  hasNextPage: boolean
  isFetchingNextPage: boolean
  isFetchNextPageError: boolean
  fetchNextPage: () => void
  refetch: () => void
} {
  const query = useInfiniteQuery(
    PostService.method.listPosts,
    {
      pageSize: POSTS_PAGE_SIZE,
      pageToken: '',
      query: narrowing.q ?? '',
      status: narrowing.status ?? '',
    },
    {
      pageParamKey: 'pageToken',
      getNextPageParam: (last) => last.nextPageToken || undefined,
      // A new narrowing keeps the rows on screen until its own answer arrives, rather than
      // blanking the list to the loading text on every settled keystroke.
      placeholderData: keepPreviousData,
      gcTime: POSTS_LIST_GC_MS,
      // Every loaded page is refetched in turn while one of its rows has a job running; a page is
      // twenty short rows and the poll stops with the job.
      refetchInterval: (state) =>
        state.state.data?.pages.some((page) => page.posts.some((post) => post.activeJob))
          ? POLL_INTERVAL_MS
          : false,
    },
  )
  const { data, fetchNextPage, refetch } = query

  // A post edited between two page fetches moves above the cursor, and the next page would
  // answer it a second time until the pages are refetched together.
  const posts = useMemo(() => {
    const seen = new Set<string>()
    const rows: PostListItem[] = []
    for (const page of data?.pages ?? []) {
      for (const summary of page.posts) {
        if (seen.has(summary.slug)) continue
        seen.add(summary.slug)
        rows.push(toPostListItem(summary))
      }
    }
    return rows
  }, [data])

  return {
    posts,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    hasNextPage: query.hasNextPage,
    isFetchingNextPage: query.isFetchingNextPage,
    isFetchNextPageError: query.isFetchNextPageError,
    fetchNextPage: () => void fetchNextPage(),
    refetch: () => void refetch(),
  }
}
