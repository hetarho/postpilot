import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { purposesQueryKey } from './purpose-queries'

/** Re-reads the directory after a create, an edit or a delete.
 *
 *  Invalidation rather than a hand-patched cache entry: `post_count` is a server-side
 *  projection over posts, so an inserted or renamed row's neighbours can change too, and
 *  guessing the new counts here would put a number in the delete confirmation that the
 *  database never agreed to. */
export function invalidatePurposes(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
): void {
  void queryClient.invalidateQueries({ queryKey: purposesQueryKey(transport, ownerId) })
}
