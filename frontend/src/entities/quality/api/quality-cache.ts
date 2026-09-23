import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'

/** Marks every quality entry stale, for every account and post on this transport. A content
 *  save, a URL save and a delete each change what some reading says — a post's own numbers, or
 *  the published window M2 and the aggregate read (QUAL-3, POST-85) — and the post hooks that
 *  call this hold no owner id. A stale neighbour costs one refetch when it is next shown. */
export function invalidateQuality(queryClient: QueryClient, transport: Transport): Promise<void> {
  return queryClient.invalidateQueries({ queryKey: ['quality', transport] })
}
