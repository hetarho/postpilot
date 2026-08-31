import { useQuery } from '@connectrpc/connect-query'
import { AuthService } from '@/shared/api'
import { SESSION_STALE_MS } from '@/shared/config'
import type { SessionUser } from '../model/types'
import { toSessionUser } from './session-queries'

/** The current session.
 *
 *  `retry: false` because a 401 is a real answer, not a transient failure worth
 *  retrying; the same staleness window the route guard uses, so the two never disagree
 *  about whether the cached session is still to be trusted. */
export function useSession(): {
  user: SessionUser | undefined
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(
    AuthService.method.getMe,
    {},
    { retry: false, staleTime: SESSION_STALE_MS },
  )
  return { user: toSessionUser(data), isPending, isError }
}
