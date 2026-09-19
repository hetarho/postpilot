/** What this screen reads out of its own address. The route validates with it, so the schema
 *  and the page that consumes it move together (ARCH-16). */
export const searchSchema = (
  search: Record<string, unknown>,
): { authKey?: string; customerKey?: string; redirect?: string } => ({
  authKey: typeof search.authKey === 'string' ? search.authKey : undefined,
  customerKey: typeof search.customerKey === 'string' ? search.customerKey : undefined,
  redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
})
