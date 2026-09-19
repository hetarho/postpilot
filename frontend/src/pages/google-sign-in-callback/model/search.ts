/** What this screen reads out of its own address. The route validates with it, so the schema
 *  and the page that consumes it move together (ARCH-16). */
export const searchSchema = (
  search: Record<string, unknown>,
): { code?: string; state?: string; error?: string } => ({
  code: typeof search.code === 'string' ? search.code : undefined,
  state: typeof search.state === 'string' ? search.state : undefined,
  error: typeof search.error === 'string' ? search.error : undefined,
})
