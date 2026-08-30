// The GetMe cache entry, reachable from React and from the router.
//
// The route guard runs outside React, so it cannot use the hooks — but it must read and
// write the SAME cache entry the hooks do, or logging in would populate one entry while
// the guard keeps checking another.
import { type Transport, Code, ConnectError } from '@connectrpc/connect'
import { createConnectQueryKey, createQueryOptions } from '@connectrpc/connect-query'
import type { QueryClient } from '@tanstack/react-query'
import { AuthService, type User } from '@/shared/api'
import { SESSION_STALE_MS } from '@/shared/config'
import type { SessionUser } from '../model/types'

/** What the server says about the caller right now.
 *
 *  "Signed out" is a state, not an error, so callers branch on it instead of catching.
 *  A real failure (the API is down) still throws — the two must not be confused, or an
 *  outage would look like a logout and send the user to a login form that cannot work.
 */
export type SessionState = { status: 'active'; user: SessionUser } | { status: 'signed-out' }

const SIGNED_OUT: SessionState = { status: 'signed-out' }

/** Maps the wire message to the app's vocabulary. */
export function toSessionUser(user: User | undefined): SessionUser | undefined {
  return user ? { id: user.id } : undefined
}

/** The exact key `useQuery(getMe, {})` registers under.
 *
 *  All four properties are load-bearing. `transport` is matched by object identity and
 *  `input` defaults to nothing rather than `{}`, so a key missing either one still works
 *  as a partial filter but silently fails to match `setQueryData`/`getQueryData` — which
 *  is how "login succeeded but the guard still bounces me" happens. */
export function getMeQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: AuthService.method.getMe,
    input: {},
    transport,
    cardinality: 'finite',
  })
}

function getMeQueryOptions(transport: Transport) {
  return createQueryOptions(AuthService.method.getMe, {}, { transport })
}

function isUnauthenticated(err: unknown): boolean {
  return ConnectError.from(err).code === Code.Unauthenticated
}

/** Resolves the caller's session for a route guard.
 *
 *  Throws only for failures that are not an answer — an outage must reach the error
 *  boundary rather than be reported as "signed out".
 */
export async function loadSession(
  queryClient: QueryClient,
  transport: Transport,
): Promise<SessionState> {
  // A 401 already in the cache is an answer; re-asking would cost a second GetMe on
  // every unauthenticated page load, since the guard that just bounced the user to
  // /login has already asked. Any OTHER cached error is not an answer, so it falls
  // through to a fresh attempt below.
  const cached = queryClient.getQueryState(getMeQueryKey(transport))
  if (cached?.status === 'error' && isUnauthenticated(cached.error)) return SIGNED_OUT

  try {
    const response = await queryClient.query({
      ...getMeQueryOptions(transport),
      // Bounded, not infinite: a session revoked elsewhere (another tab, an expiry, the
      // operator) must stop granting access without waiting for a full reload.
      staleTime: SESSION_STALE_MS,
      // The app QueryClient defaults to retry: 1, which would double every 401.
      retry: false,
    })
    const user = toSessionUser(response.user)
    // A 200 carrying no user is not a session. Treating it as one would let a malformed
    // or half-migrated response open every protected route.
    return user ? { status: 'active', user } : SIGNED_OUT
  } catch (err) {
    if (isUnauthenticated(err)) return SIGNED_OUT
    throw err
  }
}
