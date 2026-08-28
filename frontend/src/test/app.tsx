// Mounts the real app shell for a test.
import type { Transport } from '@connectrpc/connect'
import { render } from '@testing-library/react'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { routeTree } from '@/app/routes/router'
import { type FakeAuthOptions, createFakeAuthTransport, createTestQueryClient } from './session'

/** Renders the REAL route tree at `at` against a fake backend. Nothing about the routing
 *  is stubbed, so the guard, the redirects and the URL a screen navigates to are the
 *  things actually under test. Pass `transport` to supply a backend the fake cannot
 *  model (an API that is down, say). */
export function renderAppAt(
  at: string,
  options: FakeAuthOptions & { transport?: Transport } = {},
) {
  const transport = options.transport ?? createFakeAuthTransport(options)
  const queryClient: QueryClient = createTestQueryClient()
  const router = createRouter({
    routeTree,
    context: { queryClient, transport },
    history: createMemoryHistory({ initialEntries: [at] }),
  })

  render(
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>,
  )

  return { router, transport, queryClient }
}
