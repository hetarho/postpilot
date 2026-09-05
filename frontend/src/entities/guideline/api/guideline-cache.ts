import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { guidelineCandidatesQueryKey, guidelinesQueryKey } from './guideline-queries'

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
  // The candidate list too: a create is also an approval (change 26), which moves a row out of
  // the 후보 section — and the create the revision dialog runs is the same call.
  void queryClient.invalidateQueries({ queryKey: guidelineCandidatesQueryKey(transport, ownerId) })
}

/** A dismissal touches only the candidate list: nothing is saved, so the guideline list cannot
 *  have gone stale. */
export function invalidateGuidelineCandidates(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
): void {
  void queryClient.invalidateQueries({ queryKey: guidelineCandidatesQueryKey(transport, ownerId) })
}
