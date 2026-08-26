import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { transport } from '@/shared/api'
import { queryClient } from './query-client'
import { router } from './router'

/** TransportProvider sits OUTSIDE QueryClientProvider (connect-query v2's documented
 *  order) so query keys resolve against the single shared transport instance. */
export function App() {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>
  )
}
