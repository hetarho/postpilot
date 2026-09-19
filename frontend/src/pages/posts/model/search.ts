/** The narrowing the list is showing, as an address (POST-67): opening a post and coming back,
 *  a reload and a shared link all keep it. Anything the list cannot honour is dropped rather than
 *  refused — a hand-edited URL should show the list, not an error. The route validates with this,
 *  so the schema and the page that consumes it move together (ARCH-16). */
export const searchSchema = (
  search: Record<string, unknown>,
): { q?: string; status?: 'draft' | 'review' | 'finalized' } => ({
  q: typeof search.q === 'string' && search.q !== '' ? search.q : undefined,
  status:
    search.status === 'draft' || search.status === 'review' || search.status === 'finalized'
      ? search.status
      : undefined,
})
