import { QueryClient } from '@tanstack/react-query'

/** The app-wide single QueryClient (owned by the app layer). connect-query sits on top
 *  of it and turns each RPC into a declarative query. These defaults are a conservative
 *  safety net — per-query policy overrides them at the call site. */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 60_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})
