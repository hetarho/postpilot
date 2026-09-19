/** `redirect` is carried through, not consumed: a session that expired on /posts/x and then read
 *  /about must still land back on /posts/x after logging in. About never reads it for itself — it
 *  only hands it back to /login, which is the one route allowed to decide where it points. */
export const searchSchema = (search: Record<string, unknown>): { redirect?: string } => ({
  redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
})
