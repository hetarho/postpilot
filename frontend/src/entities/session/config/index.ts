/** How long a resolved session is trusted before the route guard re-checks it with the
 *  server. Zero would cost a round trip on every navigation; Infinity would let a
 *  session revoked elsewhere (another tab, an expiry, an operator) keep rendering
 *  signed-in screens until the user reloads. The app-wide QUERY_STALE_MS is deliberately
 *  twice this: the session is the one query whose staleness gates navigation, so it
 *  re-checks while ordinary data is still trusted. */
export const SESSION_STALE_MS = 30_000
