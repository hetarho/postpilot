import { QueryClient } from '@tanstack/react-query'
import { QUERY_RETRY_COUNT, QUERY_STALE_MS } from '@/shared/config'

/** The app-wide single QueryClient (owned by the app layer). connect-query sits on top
 *  of it and turns each RPC into a declarative query. These defaults are a conservative
 *  safety net — per-query policy overrides them at the call site. */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: QUERY_STALE_MS,
      retry: QUERY_RETRY_COUNT,
      // Not a tunable number but a behavioural choice: refetching on focus would repoll
      // durable server-side jobs every time the user glances at another window.
      refetchOnWindowFocus: false,
    },
  },
})
