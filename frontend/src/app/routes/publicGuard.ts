import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { loadSession } from '@/entities/session'

/** Resolves the reverse guard shared by public credential-entry pages. An API outage is
 *  swallowed so the visitor still reaches the form that can explain the failure. */
export async function hasActivePublicSession(context: {
  queryClient: QueryClient
  transport: Transport
}): Promise<boolean> {
  return loadSession(context.queryClient, context.transport)
    .then((session) => session.status === 'active')
    .catch(() => false)
}
