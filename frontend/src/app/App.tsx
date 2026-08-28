import { useEffect } from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { transport } from '@/shared/api'
import { registerAuthRedirect } from './providers/auth-redirect'
import { queryClient } from './providers/query-client'
import { router } from './routes/router'

/** TransportProvider sits OUTSIDE QueryClientProvider (connect-query v2's documented
 *  order) so query keys resolve against the single shared transport instance. */
export function App() {
  // Registered in an effect (not at module scope) so it unsubscribes cleanly — React
  // StrictMode mounts twice in dev and would otherwise leave a duplicate listener.
  useEffect(() => registerAuthRedirect({ router, queryClient, transport }), [])

  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>
  )
}
