/** Whether a presigned URL's own lifetime had already run out, answered from the URL and a clock
 *  rather than from a response.
 *
 *  This is not a shortcut, it is the only thing that works: R2 answers an expired read with a 403
 *  that carries **no CORS headers**, so the browser withholds it and both `fetch` and a
 *  CORS-loaded `<img>` fail — exactly as they do when the bucket allows this origin no `GET` at
 *  all (Cloudflare, *R2 → CORS → Use CORS with a presigned URL*). The two failures are therefore
 *  indistinguishable from the response, and they lead to opposite advice: reload the post, or
 *  stop trying and place the photo by hand.
 *
 *  SigV4 states the lifetime in the query (`X-Amz-Date` + `X-Amz-Expires`), so no request is
 *  needed. A URL that carries neither is not presigned and cannot be claimed to have expired. A
 *  wrong client clock costs a less useful message and nothing else — nothing here is a security
 *  decision. */
export function presignExpired(url: string, now: number): boolean {
  let query: URLSearchParams
  try {
    query = new URL(url).searchParams
  } catch {
    return false
  }
  const lifetimeSeconds = Number(query.get('X-Amz-Expires'))
  if (!Number.isFinite(lifetimeSeconds) || lifetimeSeconds <= 0) return false
  // `20260904T010203Z` — ISO 8601 *basic* format, which `Date.parse` is not required to accept.
  const signedAt = Date.parse(
    (query.get('X-Amz-Date') ?? '').replace(
      /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/,
      '$1-$2-$3T$4:$5:$6Z',
    ),
  )
  if (Number.isNaN(signedAt)) return false
  return now > signedAt + lifetimeSeconds * 1000
}
