import { useCallback } from 'react'
import type { Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import { getPostQueryKey, listPostsQueryKey, postDetailQueriesKey } from './post-queries'

/** Which other noun a write touched. A post embeds its voice and its template by reference and
 *  renders their names, and a guideline decides what a post was written under — so a write to any
 *  of the three leaves every cached post displaying something the server no longer holds. */
export interface PostDependency {
  voiceId?: string
  templateId?: string
  guidelineId?: string
}

/** The ONE place that fact is stated (ARCH-14). Before this, each verb repeated `listPostsQueryKey`
 *  + `postDetailQueriesKey` in its own `onSuccess`, so the next voice or template verb had to
 *  remember the rule, and a post key-shape change edited eight slices.
 *
 *  Every kind invalidates the same two entries, and deliberately so: the post cache is partitioned
 *  by slug, not by voice or template, so which posts name the changed noun is not knowable from
 *  the client's keys. The parameter records WHY the posts went stale — it is what a reader (and a
 *  future per-dependency partition) needs, not a filter the cache can apply today. */
export function invalidatePostsDependingOn(
  queryClient: QueryClient,
  transport: Transport,
  dependency: PostDependency,
): void {
  if (!dependency.voiceId && !dependency.templateId && !dependency.guidelineId) return
  void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
  void queryClient.invalidateQueries({ queryKey: postDetailQueriesKey(transport) })
}

/** A write that produced a post's own content — an applied experiment winner, say. Its detail
 *  entry and the list that shows its status are stale; no other post is. */
export function invalidateWrittenPost(
  queryClient: QueryClient,
  transport: Transport,
  slug: string,
): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) }),
    ...(slug
      ? [queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, slug) })]
      : []),
  ])
}

/** The same entry for a caller that holds no transport of its own (ARCH-17). */
export function useInvalidatePostsDependingOn(): (dependency: PostDependency) => void {
  const transport = useTransport()
  const queryClient = useQueryClient()
  return useCallback(
    (dependency: PostDependency) => invalidatePostsDependingOn(queryClient, transport, dependency),
    [queryClient, transport],
  )
}
