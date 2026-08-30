export {
  transport,
  authClient,
  healthClient,
  providerClient,
  generationClient,
  voiceClient,
  voiceLearningClient,
  voiceValidationClient,
  modelExperimentClient,
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
  BlockSchema,
  BlockType,
  ConfirmUploadResponseSchema,
  CreateUploadResponseSchema,
  DeleteImageResponseSchema,
  GetPostResponseSchema,
  GetGenerationResponseSchema,
  GenerationJobSchema,
  ImageSchema,
  ListPostsResponseSchema,
  ObservationSchema,
  PostContentSchema,
  PostSchema,
  PostSummarySchema,
  SavePostDraftResponseSchema,
  SavePostContentResponseSchema,
  SavePostGenerationOptionsResponseSchema,
  FinalizePostResponseSchema,
  StartGenerationRequestSchema,
  StartGenerationResponseSchema,
  StartRevisionRequestSchema,
  StartRevisionResponseSchema,
} from './gen/postpilot/v1/post_pb'
export type {
  Block,
  GenerationJob as ProtoGenerationJob,
  GetGenerationResponse,
  GetPostResponse,
  Image,
  Observation,
  Post,
  PostContent,
  PostSummary,
  StartGenerationRequest,
  StartGenerationResponse,
  StartRevisionRequest,
  StartRevisionResponse,
  SavePostContentResponse,
} from './gen/postpilot/v1/post_pb'
export { ProviderService, Stage, SelectionSlot } from './gen/postpilot/v1/provider_pb'
export {
  GetSelectionsResponseSchema,
  ListModelsResponseSchema,
  ModelInfoSchema,
  ModelRefSchema,
  SaveSelectionResponseSchema,
  SelectionSchema,
  ComparisonPairSchema,
  RecommendationSetSchema,
  GetComparisonPairsResponseSchema,
  ListRecommendationSetsResponseSchema,
  SaveComparisonPairResponseSchema,
  ApplyRecommendationSetResponseSchema,
} from './gen/postpilot/v1/provider_pb'
export { VoiceService } from './gen/postpilot/v1/voice_pb'
export {
  AddVoiceSampleResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  UpdateVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceSampleSchema,
  StructuredVoiceProfileSchema,
  VoiceProfileVersionSchema,
  ListVoiceProfileVersionsResponseSchema,
  UpdateVoiceOverrideResponseSchema,
  VoiceLayer,
  VoiceRuleStatus,
  VoiceValueSource,
} from './gen/postpilot/v1/voice_pb'
export type {
  GetVoiceProfileResponse,
  VoiceProfile as ProtoVoiceProfile,
  VoiceSample as ProtoVoiceSample,
  StructuredVoiceProfile,
  VoiceProfileVersion,
} from './gen/postpilot/v1/voice_pb'
export {
  VoiceLearningService,
  VoiceFeedbackReason,
  VoiceLearningEventSchema,
  LearnFromFinalizedPostResponseSchema,
  RetryVoiceLearningResponseSchema,
  GiveSentenceFeedbackResponseSchema,
  ListRuleConfirmationsResponseSchema,
} from './gen/postpilot/v1/voice_learning_pb'
export type { VoiceLearningEvent } from './gen/postpilot/v1/voice_learning_pb'
export {
  VoiceValidationService,
  ListVoiceProfileValidationsResponseSchema,
} from './gen/postpilot/v1/voice_validation_pb'
export type { VoiceRuleComparison, VoiceProfileValidation } from './gen/postpilot/v1/voice_validation_pb'
export type {
  GetComparisonPairsResponse,
  GetSelectionsResponse,
  ModelInfo as ProtoModelInfo,
  ModelRef as ProtoModelRef,
  Selection as ProtoSelection,
  ComparisonPair as ProtoComparisonPair,
  RecommendationSet as ProtoRecommendationSet,
} from './gen/postpilot/v1/provider_pb'
export {
  ModelExperimentService,
  ExperimentStatus,
  DisplaySide,
  CandidateStatus,
  ExperimentOutcome,
  CostSource,
  StartExperimentResponseSchema,
} from './gen/postpilot/v1/model_experiment_pb'
export type {
  ModelExperiment as ProtoModelExperiment,
  ExperimentCandidate as ProtoExperimentCandidate,
  LeaderboardEntry as ProtoLeaderboardEntry,
} from './gen/postpilot/v1/model_experiment_pb'
