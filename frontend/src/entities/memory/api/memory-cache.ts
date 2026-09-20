import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { memoriesQueryKey } from './memory-queries'

/** Re-reads the list after a create, an edit or a delete. Invalidation rather than a
 *  hand-patched entry because the list's ORDER is the server's — a create can also be a second
 *  sighting of a text the account already holds, which moves an existing row rather than adding
 *  one (MEM-9). */
export function invalidateMemories(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
): void {
  void queryClient.invalidateQueries({ queryKey: memoriesQueryKey(transport, ownerId) })
}
