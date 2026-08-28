export {
  transport,
  authClient,
  healthClient,
  credentialedFetch,
  unauthenticatedInterceptor,
} from './transport'
export { onUnauthenticated, emitUnauthenticated } from './auth-events'

// The generated module is re-exported ONLY from here (ARCHITECTURE §3.3): a proto
// rename stops at this directory instead of rippling through the slices.
export { AuthService } from './gen/postpilot/v1/auth_pb'
export {
  GetMeResponseSchema,
  LoginResponseSchema,
  LogoutResponseSchema,
} from './gen/postpilot/v1/auth_pb'
export type { User, GetMeResponse } from './gen/postpilot/v1/auth_pb'
export { HealthService, PingResponseSchema } from './gen/postpilot/v1/health_pb'
export type { PingResponse } from './gen/postpilot/v1/health_pb'
