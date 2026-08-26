// The Connect transport and the typed HealthService client.
//
// In dev, baseUrl '/api' is rewritten to the backend root by the Vite proxy
// (vite.config.ts), so calls hit '/postpilot.v1.HealthService/Ping' with no CORS.
// Built assets (preview/prod) have no such proxy, so they target the backend origin
// from VITE_API_URL.
import { createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { API_URL } from '@/shared/config'
import { HealthService } from './gen/postpilot/v1/health_pb'

const baseUrl = import.meta.env.DEV ? '/api' : API_URL

/** The single Connect transport. Exported so connect-query can mount it on
 *  TransportProvider and build query keys against the SAME instance the imperative
 *  client uses — two transports would split the query-key space. */
export const transport = createConnectTransport({ baseUrl })

/** Typed client for postpilot.v1.HealthService. */
export const healthClient = createClient(HealthService, transport)
