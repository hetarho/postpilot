import { useEffect, useRef } from 'react'
import { completePostHistoryReturn, readPostHistoryReturn } from '@/entities/post'
import type { PostNarrowing } from '@/features/filter-posts'

interface HistoryList {
  posts: readonly { slug: string }[]
  isPending: boolean
  isFetching: boolean
  isError: boolean
  hasNextPage: boolean
  isFetchingNextPage: boolean
  isFetchNextPageError: boolean
  fetchNextPage: () => void
}

/** An explicit editor return may precede a cold history cache. Wait for its own
 * narrowing, then load enough rows to restore the saved position in the document.
 * A failed request keeps the return pending until the owner's retry succeeds. */
export function useHistoryScrollReturn(
  ownerId: string,
  narrowing: PostNarrowing,
  settledQ: string,
  list: HistoryList,
) {
  const requestedPage = useRef<string | undefined>(undefined)
  const q = narrowing.q ?? ''
  const status = narrowing.status ?? ''
  const {
    posts,
    isPending,
    isFetching,
    isError,
    hasNextPage,
    isFetchingNextPage,
    isFetchNextPageError,
    fetchNextPage,
  } = list

  useEffect(() => {
    if (isError || isFetchNextPageError) requestedPage.current = undefined
    if (
      !ownerId ||
      settledQ !== q ||
      isPending ||
      isFetching ||
      isError ||
      isFetchingNextPage ||
      isFetchNextPageError
    )
      return
    const entry = readPostHistoryReturn(ownerId)
    if (!entry || (entry.filters.q ?? '') !== q || (entry.filters.status ?? '') !== status) return
    const identity = JSON.stringify([ownerId, q, status, entry.targetId, entry.scrollY])

    const frame = requestAnimationFrame(() => {
      const current = readPostHistoryReturn(ownerId)
      if (
        !current ||
        JSON.stringify([
          ownerId,
          current.filters.q ?? '',
          current.filters.status ?? '',
          current.targetId,
          current.scrollY,
        ]) !== identity
      )
        return
      const documentHeight = Math.max(
        document.documentElement.scrollHeight,
        document.body.scrollHeight,
      )
      const positionAvailable = documentHeight - window.innerHeight >= entry.scrollY
      // Editing moves a target to the newest row. Its presence does not mean the
      // older rows needed for the retained document position have loaded yet.
      if (!positionAvailable && hasNextPage) {
        const page = JSON.stringify([identity, posts.length, posts.at(-1)?.slug])
        if (requestedPage.current !== page) {
          requestedPage.current = page
          fetchNextPage()
        }
        return
      }
      // Exhaustion also completes a deleted-target return; the browser clamps the
      // saved position to the remaining document after the rows have been painted.
      window.scrollTo(0, entry.scrollY)
      completePostHistoryReturn(ownerId)
      requestedPage.current = undefined
    })
    return () => cancelAnimationFrame(frame)
  }, [
    ownerId,
    q,
    status,
    settledQ,
    posts,
    isPending,
    isFetching,
    isError,
    hasNextPage,
    isFetchingNextPage,
    isFetchNextPageError,
    fetchNextPage,
  ])
}
