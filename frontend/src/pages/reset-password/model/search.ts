/** What this screen reads out of its own address. The route validates with it, so the schema
 *  and the page that consumes it move together (ARCH-16). */
export const searchSchema = (search: Record<string, unknown>): { token?: string } => ({
  token: typeof search.token === 'string' ? search.token : undefined,
})
