export {
  transport,
  authClient,
  healthClient,
  providerClient,
  generationClient,
  voiceClient,
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
export { GenerationService, PostService } from './gen/postpilot/v1/post_pb'
export {
  ConfirmUploadResponseSchema,
  CreateUploadResponseSchema,
  DeleteImageResponseSchema,
  GetPostResponseSchema,
  GetGenerationResponseSchema,
  GenerationJobSchema,
  ImageSchema,
  ListPostsResponseSchema,
  PostSchema,
  PostSummarySchema,
  SavePostDraftResponseSchema,
} from './gen/postpilot/v1/post_pb'
export type {
  GenerationJob as ProtoGenerationJob,
  GetGenerationResponse,
  GetPostResponse,
  Image,
  Post,
  PostSummary,
} from './gen/postpilot/v1/post_pb'
export { ProviderService, Stage } from './gen/postpilot/v1/provider_pb'
export {
  GetSelectionsResponseSchema,
  ListModelsResponseSchema,
  ModelInfoSchema,
  ModelRefSchema,
  SaveSelectionResponseSchema,
  SelectionSchema,
} from './gen/postpilot/v1/provider_pb'
export { VoiceService } from './gen/postpilot/v1/voice_pb'
export {
  AddVoiceSampleResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  UpdateVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceSampleSchema,
} from './gen/postpilot/v1/voice_pb'
export type {
  GetVoiceProfileResponse,
  VoiceProfile as ProtoVoiceProfile,
  VoiceSample as ProtoVoiceSample,
} from './gen/postpilot/v1/voice_pb'
export type {
  GetSelectionsResponse,
  ModelInfo as ProtoModelInfo,
  ModelRef as ProtoModelRef,
  Selection as ProtoSelection,
} from './gen/postpilot/v1/provider_pb'
