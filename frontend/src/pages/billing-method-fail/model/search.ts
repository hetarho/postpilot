/** What this screen reads out of its own address. The route validates with it, so the schema
 *  and the page that consumes it move together (ARCH-16). */
export const searchSchema = (
  search: Record<string, unknown>,
): { code?: string; message?: string; redirect?: string } => ({
  code: typeof search.code === 'string' ? search.code : undefined,
  message: typeof search.message === 'string' ? search.message : undefined,
  redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
})
