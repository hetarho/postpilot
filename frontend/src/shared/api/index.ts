export {
  transport,
  authClient,
  healthClient,
  providerClient,
  modelCatalogClient,
  generationClient,
  publishingClient,
  publishingClientFor,
  purposeClient,
  voiceClient,
  voiceLearningClient,
  voiceValidationClient,
  modelExperimentClient,
  credentialedFetch,
  unauthenticatedInterceptor,
} from './transport'
export { onUnauthenticated, emitUnauthenticated } from './auth-events'
export {
  contentLanguages,
  contentLanguageFromProto,
  contentLanguageToProto,
  requireContentLanguage,
} from './language'
export type { ContentLanguage } from './language'
export {
  appFailureFromConnect,
  appFailureFromProto,
  appFailureSpecs,
  normalizeAppFailure,
} from './app-failure'
export type { AppFailure, AppFailureReason } from './app-failure'
export { AppErrorDetailSchema, FailureSchema } from './gen/postpilot/v1/error_pb'
export type { Failure as ProtoFailure } from './gen/postpilot/v1/error_pb'

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
export { AdminService, Plan as ProtoPlan, PlanService } from './gen/postpilot/v1/plan_pb'
export {
  GetMyPlanResponseSchema,
  ListUsersResponseSchema,
  SetUserPlanResponseSchema,
} from './gen/postpilot/v1/plan_pb'
export type {
  GetMyPlanResponse,
  CreditBalance as ProtoCreditBalance,
  CreditLot as ProtoCreditLot,
  PlanUser as ProtoPlanUser,
} from './gen/postpilot/v1/plan_pb'
export type { PingResponse } from './gen/postpilot/v1/health_pb'
export { GenerationService, PostService } from './gen/postpilot/v1/post_pb'
export {
  BlockSchema,
  BlockType,
  ConfirmUploadResponseSchema,
  CreateUploadResponseSchema,
  DeleteImageResponseSchema,
  DeletePostResponseSchema,
  GetPostResponseSchema,
  GetGenerationResponseSchema,
  GenerationJobSchema,
  ImageSchema,
  ListPostsResponseSchema,
  ObservationSchema,
  PostContentSchema,
  PostSchema,
  PostSummarySchema,
  ReobserveSelectionSchema,
  SavePostDraftResponseSchema,
  SavePostContentResponseSchema,
  SavePostGenerationOptionsResponseSchema,
  FinalizePostResponseSchema,
  StartGenerationRequestSchema,
  StartGenerationResponseSchema,
  StartRevisionRequestSchema,
  StartRevisionResponseSchema,
  VoiceRefSchema,
} from './gen/postpilot/v1/post_pb'
export type {
  Block,
  VoiceRef as ProtoVoiceRef,
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
export { ModelCatalogService } from './gen/postpilot/v1/model_catalog_pb'
export {
  CatalogEntrySchema,
  ListCatalogResponseSchema,
  SetModelPurposeResponseSchema,
  UpdateModelResponseSchema,
} from './gen/postpilot/v1/model_catalog_pb'
export type {
  CatalogEntry as ProtoCatalogEntry,
  ListCatalogResponse as ProtoListCatalogResponse,
} from './gen/postpilot/v1/model_catalog_pb'
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
  GetVoiceProfileVersionSampleResponseSchema,
  RestoreVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceSampleSchema,
  StructuredVoiceProfileSchema,
  VoiceProfileVersionSchema,
  ListVoiceProfileVersionsResponseSchema,
  UpdateVoiceOverrideResponseSchema,
  VoiceLayer,
  VoiceRuleStatus,
  VoiceValueSource,
  VoiceSchema,
  ListVoicesResponseSchema,
  CreateVoiceResponseSchema,
  RenameVoiceResponseSchema,
  SetDefaultVoiceResponseSchema,
  DeleteVoiceResponseSchema,
  RestoreVoiceResponseSchema,
} from './gen/postpilot/v1/voice_pb'
export type {
  GetVoiceProfileResponse,
  ListVoicesResponse,
  Voice as ProtoVoice,
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
export type {
  VoiceRuleComparison,
  VoiceProfileValidation,
} from './gen/postpilot/v1/voice_validation_pb'
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
  ListExperimentsResponseSchema,
  StartExperimentResponseSchema,
} from './gen/postpilot/v1/model_experiment_pb'
export {
  PublishingService,
  PublishVisibility,
  PublishStatus,
  PublishStage,
  PublishingAgentSchema,
  PublishJobSchema,
  CreateAgentPairingResponseSchema,
  ListPublishingAgentsResponseSchema,
  UpdatePublishingAgentResponseSchema,
  RevokePublishingAgentResponseSchema,
  StartPublishResponseSchema,
  GetPublishJobResponseSchema,
  ListRetryablePublishJobsResponseSchema,
  RetryPublishResponseSchema,
  CancelPublishResponseSchema,
} from './gen/postpilot/v1/publishing_pb'
export type {
  PublishingAgent as ProtoPublishingAgent,
  PublishingCategory as ProtoPublishingCategory,
  PublishJob as ProtoPublishJob,
} from './gen/postpilot/v1/publishing_pb'
export {
  GuidelineService,
  GuidelineSchema,
  GuidelinePurposeRefSchema,
  GuidelineScope as ProtoGuidelineScope,
  ListGuidelinesResponseSchema,
  CreateGuidelineResponseSchema,
  UpdateGuidelineResponseSchema,
  DeleteGuidelineResponseSchema,
} from './gen/postpilot/v1/guideline_pb'
export type {
  Guideline as ProtoGuideline,
  GuidelinePurposeRef as ProtoGuidelinePurposeRef,
} from './gen/postpilot/v1/guideline_pb'
export {
  PurposeService,
  PurposeSchema,
  PurposeRefSchema,
  ListPurposesResponseSchema,
  CreatePurposeResponseSchema,
  UpdatePurposeResponseSchema,
  DeletePurposeResponseSchema,
} from './gen/postpilot/v1/purpose_pb'
export type {
  Purpose as ProtoPurpose,
  PurposeRef as ProtoPurposeRef,
} from './gen/postpilot/v1/purpose_pb'
export type {
  ModelExperiment as ProtoModelExperiment,
  ExperimentCandidate as ProtoExperimentCandidate,
  LeaderboardEntry as ProtoLeaderboardEntry,
} from './gen/postpilot/v1/model_experiment_pb'
