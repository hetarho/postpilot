import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { guidelinesQueryKey } from './guideline-queries'

/** Re-reads the list after a create, an edit or a delete — and after a purpose is renamed or
 *  deleted, because each scoped guideline's chips are a PROJECTION of purpose names, not a
 *  column of its own row. Invalidation rather than a hand-patched entry for the same reason:
 *  the list's order is the injection order the server decides. */
export function invalidateGuidelines(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
): void {
  void queryClient.invalidateQueries({ queryKey: guidelinesQueryKey(transport, ownerId) })
}
