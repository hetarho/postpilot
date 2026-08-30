// The Connect transport and the typed clients.
//
// In dev, baseUrl '/api' is rewritten to the backend root by the Vite proxy
// (vite.config.ts), so calls hit '/postpilot.v1.HealthService/Ping' with no CORS.
// Built assets (preview/prod) have no such proxy, so they target the backend origin
// from VITE_API_URL.
import {
  Code,
  ConnectError,
  createClient,
  type Interceptor,
  type Transport,
} from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { API_URL } from '@/shared/config'
import { emitUnauthenticated } from './auth-events'
import { AuthService } from './gen/postpilot/v1/auth_pb'
import { HealthService } from './gen/postpilot/v1/health_pb'
import { ModelExperimentService } from './gen/postpilot/v1/model_experiment_pb'
import { GenerationService } from './gen/postpilot/v1/post_pb'
import { ProviderService } from './gen/postpilot/v1/provider_pb'
import { PublishingService } from './gen/postpilot/v1/publishing_pb'
import { PurposeService } from './gen/postpilot/v1/purpose_pb'
import { VoiceService } from './gen/postpilot/v1/voice_pb'
import { VoiceLearningService } from './gen/postpilot/v1/voice_learning_pb'
import { VoiceValidationService } from './gen/postpilot/v1/voice_validation_pb'

const baseUrl = import.meta.env.DEV ? '/api' : API_URL

/** Sends the session cookie on every call.
 *
 *  The session is an HttpOnly cookie on a different origin than the SPA, so the browser
 *  omits it unless the request opts in. connect-web has no `credentials` option, so the
 *  opt-in rides on a fetch wrapper. Exported because a `Transport` hides its options —
 *  this function is the only testable surface for the flag. */
export const credentialedFetch: typeof globalThis.fetch = (input, init) =>
  globalThis.fetch(input, { ...init, credentials: 'include' })

/** Turns a 401 into an app-wide "your session is gone" event.
 *
 *  Login is the one exemption: its 401 means the password was wrong, which the form
 *  reports itself — announcing it as a lost session would bounce the user off the login
 *  page they are standing on. Every other 401, GetMe's included, is a session that
 *  stopped being valid, and the app has to react even when the call was a background
 *  refetch nobody is watching. */
export const unauthenticatedInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req)
  } catch (err) {
    // ConnectError.from normalizes anything (network failures included) without throwing.
    const error = ConnectError.from(err)
    const isLogin = req.service.typeName === AuthService.typeName && req.method.name === 'Login'
    if (error.code === Code.Unauthenticated && !isLogin) emitUnauthenticated()
    throw error
  }
}

/** The single Connect transport. Exported so connect-query can mount it on
 *  TransportProvider and build query keys against the SAME instance the imperative
 *  client uses — connect-query keys transports by object identity, so two transports
 *  would split the query-key space. */
export const transport = createConnectTransport({
  baseUrl,
  fetch: credentialedFetch,
  interceptors: [unauthenticatedInterceptor],
})

/** Typed client for postpilot.v1.AuthService. */
export const authClient = createClient(AuthService, transport)

/** Typed client for postpilot.v1.HealthService. */
export const healthClient = createClient(HealthService, transport)

/** Typed client for postpilot.v1.ProviderService. */
export const providerClient = createClient(ProviderService, transport)

/** Typed client for starting and reading durable generation jobs. */
export const generationClient = createClient(GenerationService, transport)

/** Human-session client for paired Mac agents and explicit publication jobs. */
export const publishingClient = createClient(PublishingService, transport)
export const publishingClientFor = (clientTransport: Transport) =>
  createClient(PublishingService, clientTransport)

/** Typed client for the acting account's reusable 용도 briefs. */
export const purposeClient = createClient(PurposeService, transport)

/** Typed client for the acting account's voice profile. */
export const voiceClient = createClient(VoiceService, transport)
export const voiceLearningClient = createClient(VoiceLearningService, transport)
export const voiceValidationClient = createClient(VoiceValidationService, transport)

/** Typed client for blind model experiments and private leaderboards. */
export const modelExperimentClient = createClient(ModelExperimentService, transport)
