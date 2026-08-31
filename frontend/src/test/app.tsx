// Mounts the real app shell for a test.
import type { Transport } from '@connectrpc/connect'
import { render } from '@testing-library/react'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { ThemeProvider, bootstrapTheme, type ThemeRuntimePorts } from '@/app/providers/theme'
import { routeTree } from '@/app/routes/router'
import { type FakeAuthOptions, createFakeAuthTransport, createTestQueryClient } from './session'
import { createThemeTestEnvironment } from './theme'

export type RenderAppOptions = FakeAuthOptions & {
  transport?: Transport
  theme?: ThemeRuntimePorts
}

/** Renders the REAL route tree at `at` against a fake backend. Nothing about the routing
 *  is stubbed, so the guard, the redirects and the URL a screen navigates to are the
 *  things actually under test. Pass `transport` to supply a backend the fake cannot
 *  model (an API that is down, say). */
export function renderAppAt(at: string, options: RenderAppOptions = {}) {
  const transport = options.transport ?? createFakeAuthTransport(options)
  const queryClient: QueryClient = createTestQueryClient()
  const fallbackTheme = createThemeTestEnvironment()
  const storage =
    options.theme?.storage === undefined ? fallbackTheme.storage : options.theme.storage
  const mediaQuery =
    options.theme?.mediaQuery === undefined ? fallbackTheme.mediaQuery : options.theme.mediaQuery
  const storageEvents =
    options.theme?.storageEvents === undefined
      ? fallbackTheme.storageEvents
      : options.theme.storageEvents
  const targetDocument =
    options.theme?.targetDocument === undefined ? document : options.theme.targetDocument
  const themeSnapshot = bootstrapTheme({
    storage,
    matchMedia: mediaQuery ? () => mediaQuery : null,
    targetDocument,
  })
  const router = createRouter({
    routeTree,
    context: { queryClient, transport },
    history: createMemoryHistory({ initialEntries: [at] }),
  })

  const view = render(
    <ThemeProvider
      initialSnapshot={themeSnapshot}
      ports={{ storage, mediaQuery, storageEvents, targetDocument }}
    >
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </TransportProvider>
    </ThemeProvider>,
  )

  return { ...view, router, transport, queryClient }
}
