export {
  transport,
  authClient,
  billingClient,
  healthClient,
  providerClient,
  modelCatalogClient,
  generationClient,
  templateClient,
  voiceClient,
  modelExperimentClient,
  credentialedFetch,
  unauthenticatedInterceptor,
} from './transport'
export { onUnauthenticated, emitUnauthenticated } from './auth-events'
export { ClipTemplateService } from './gen/postpilot/v1/clip_template_pb'
export { ClipSourceService } from './gen/postpilot/v1/clip_source_pb'
export { ClipGenerationService } from './gen/postpilot/v1/clip_generation_pb'
export { ClipPlanService } from './gen/postpilot/v1/clip_plan_pb'
export { ClipRenderService } from './gen/postpilot/v1/clip_render_pb'
export {
  ClipRenderKind,
  ClipPreviewParity,
  VideoTemplateSchema,
  ClipProjectSchema,
  ClipObservationsSchema,
  ClipEditPlanSchema,
  ClipEditingStateSchema,
  ClipSourceMetadataSchema,
  ClipSourceBatchSchema,
  ClipAccountingSchema,
  ClipAttemptSchema,
  ClipAnalysisEligibility,
} from './gen/postpilot/v1/clip_pb'
export {
  CreateClipProjectResponseSchema,
  DeleteClipProjectResponseSchema,
  GetClipProjectResponseSchema,
  ListClipAnalysisEligibilityResponseSchema,
  ListClipProjectsResponseSchema,
  QuoteClipGenerationResponseSchema,
  StartClipGenerationResponseSchema,
  UpdateClipProjectResponseSchema,
} from './gen/postpilot/v1/clip_generation_pb'
export {
  GetClipCaptionPreviewRequestSchema,
  GetClipCaptionPreviewResponseSchema,
  GetClipCaptionStyleSamplesRequestSchema,
  GetClipCaptionStyleSamplesResponseSchema,
  GetClipRegionPresetSamplesRequestSchema,
  GetClipRegionPresetSamplesResponseSchema,
  QuoteClipRevisionResponseSchema,
  SaveClipEditPlanResponseSchema,
  StartClipRevisionResponseSchema,
} from './gen/postpilot/v1/clip_plan_pb'
export {
  PrepareClipCaptionFramesRequestSchema,
  PrepareClipCaptionFramesResponseSchema,
  PrepareClipPreviewRequestSchema,
  PrepareClipPreviewResponseSchema,
  StartClipRenderResponseSchema,
} from './gen/postpilot/v1/clip_render_pb'
export {
  ConfirmClipSourceResponseSchema,
  CreateClipSourceBatchResponseSchema,
  DiscardClipSourceBatchResponseSchema,
  ReorderClipSourcesResponseSchema,
} from './gen/postpilot/v1/clip_source_pb'
export {
  CreateVideoTemplateResponseSchema,
  DeleteVideoTemplateResponseSchema,
  ListVideoTemplatesResponseSchema,
  UpdateVideoTemplateResponseSchema,
} from './gen/postpilot/v1/clip_template_pb'
export type {
  ListClipAnalysisEligibilityResponse as ProtoClipAnalysisEligibilityList,
  QuoteClipGenerationResponse as ProtoClipQuote,
} from './gen/postpilot/v1/clip_generation_pb'
export type {
  VideoTemplate as ProtoVideoTemplate,
  ClipProject as ProtoClipProject,
  ClipProjectComposition as ProtoClipProjectComposition,
  ClipCompositionInputs as ProtoClipCompositionInputs,
  ClipObservations as ProtoClipObservations,
  ClipAttemptInspection as ProtoClipAttemptInspection,
  ClipNarration as ProtoClipNarration,
  ClipEditPlan as ProtoClipEditPlan,
  ClipEditingState as ProtoClipEditingState,
  ClipSourceMetadata as ProtoClipSourceMetadata,
  ClipSourceBatch as ProtoClipSourceBatch,
  ClipSource as ProtoClipSource,
  ClipSourceUpload as ProtoClipSourceUpload,
  ClipAccounting as ProtoClipAccounting,
} from './gen/postpilot/v1/clip_pb'
export type {
  GetClipCaptionPreviewResponse as ProtoGetClipCaptionPreviewResponse,
  GetClipCaptionStyleSamplesResponse as ProtoGetClipCaptionStyleSamplesResponse,
  QuoteClipRevisionResponse as ProtoClipRevisionQuote,
} from './gen/postpilot/v1/clip_plan_pb'
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
  retriableTransportFailure,
} from './app-failure'
export type { AppFailure, AppFailureReason } from './app-failure'
export { AppErrorDetailSchema, FailureSchema } from './gen/postpilot/v1/error_pb'
export type { Failure as ProtoFailure } from './gen/postpilot/v1/error_pb'

// The generated module is re-exported ONLY from here (ARCH-17): a proto
// rename stops at this directory instead of rippling through the slices.
export { AuthService } from './gen/postpilot/v1/auth_pb'
export {
  ChangePasswordResponseSchema,
  GetMeResponseSchema,
  LoginResponseSchema,
  SignInWithGoogleResponseSchema,
  LogoutResponseSchema,
  RegisterEmailResponseSchema,
  RequestPasswordResetResponseSchema,
  ResendVerificationResponseSchema,
  ResetPasswordResponseSchema,
  SignupResponseSchema,
  VerifyEmailResponseSchema,
} from './gen/postpilot/v1/auth_pb'
export type { User, GetMeResponse } from './gen/postpilot/v1/auth_pb'
export { BillingService, Term as ProtoTerm } from './gen/postpilot/v1/billing_pb'
export {
  CancelScheduledChangeResponseSchema,
  CancelSubscriptionResponseSchema,
  ChangeSubscriptionResponseSchema,
  GetMyBillingResponseSchema,
  QuoteChangeResponseSchema,
  QuotePriceResponseSchema,
  QuotePurchaseResponseSchema,
  PurchaseCreditsResponseSchema,
  RequestRefundResponseSchema,
  ListMyRefundsResponseSchema,
  ListRefundReviewsResponseSchema,
  ReviewRefundResponseSchema,
  ReconcileRefundResponseSchema,
  RegisterPaymentMethodResponseSchema,
  RemovePaymentMethodResponseSchema,
  ResumeSubscriptionResponseSchema,
  SubscribeResponseSchema,
} from './gen/postpilot/v1/billing_pb'
export type {
  BillingSubscription as ProtoBillingSubscription,
  BillingPaymentMethod as ProtoBillingPaymentMethod,
  BillingEvent as ProtoBillingEvent,
  BillingPurchase as ProtoBillingPurchase,
  GetMyBillingResponse,
  QuoteChangeResponse,
  QuotePriceResponse,
  QuotePurchaseResponse,
  PurchaseCreditsResponse,
  BillingRefundRequest as ProtoBillingRefundRequest,
  RegisterPaymentMethodResponse,
  SubscribeResponse,
} from './gen/postpilot/v1/billing_pb'
export { HealthService, PingResponseSchema } from './gen/postpilot/v1/health_pb'
export { VoucherService, VoucherState as ProtoVoucherState } from './gen/postpilot/v1/voucher_pb'
export {
  GetVoucherResponseSchema,
  IssueVoucherResponseSchema,
  ListVouchersResponseSchema,
  RedeemVoucherResponseSchema,
  RevokeVoucherResponseSchema,
  VoucherPresetSchema,
  VoucherSchema,
} from './gen/postpilot/v1/voucher_pb'
export type {
  GetVoucherResponse,
  ListVouchersResponse,
  RedeemVoucherResponse,
  Voucher as ProtoVoucher,
  VoucherPreset as ProtoVoucherPreset,
} from './gen/postpilot/v1/voucher_pb'
export { AdminService, Plan as ProtoPlan, PlanService } from './gen/postpilot/v1/plan_pb'
export {
  GetExchangeRateResponseSchema,
  GetMyPlanResponseSchema,
  ListUsersResponseSchema,
  SetEstimatorComboResponseSchema,
  SetUserPlanResponseSchema,
} from './gen/postpilot/v1/plan_pb'
export type {
  GetExchangeRateResponse,
  GetMyPlanResponse,
  CreditBalance as ProtoCreditBalance,
  CreditLot as ProtoCreditLot,
  PlanUser as ProtoPlanUser,
} from './gen/postpilot/v1/plan_pb'
export type { PingResponse } from './gen/postpilot/v1/health_pb'
export { GenerationService, PostService } from './gen/postpilot/v1/post_pb'
export {
  AttachmentKind,
  BlogField as ProtoBlogField,
  BlockSchema,
  BlockType,
  GalleryLayout,
  ConfirmUploadResponseSchema,
  CreateUploadResponseSchema,
  DeleteImageResponseSchema,
  RotateImageResponseSchema,
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
  SavePostPublishedUrlResponseSchema,
  DeleteVideoResponseSchema,
  VideoSchema,
  StartGenerationRequestSchema,
  StartGenerationResponseSchema,
  StartRevisionRequestSchema,
  StartRevisionResponseSchema,
  StartStorylineResponseSchema,
  StartStorylineRevisionResponseSchema,
  StorylineEditSchema,
  StorylineParagraphSchema,
  StorylineSchema,
  TemplateAnswerSchema,
  VoiceRefSchema,
} from './gen/postpilot/v1/post_pb'
export type {
  Block,
  TemplateAnswer as ProtoTemplateAnswer,
  VoiceRef as ProtoVoiceRef,
  Storyline as ProtoStoryline,
  GenerationJob as ProtoGenerationJob,
  GetGenerationResponse,
  GetPostResponse,
  Image,
  Observation,
  Post,
  PostContent,
  Video,
  PostSummary,
  StartGenerationRequest,
  StartGenerationResponse,
  StartRevisionRequest,
  StartRevisionResponse,
  SavePostContentResponse,
} from './gen/postpilot/v1/post_pb'
export {
  ModelCatalogService,
  ModelPurpose as ProtoModelPurpose,
} from './gen/postpilot/v1/model_catalog_pb'
export {
  SpeechProfileService,
  SpeechProfileChoiceSchema,
  AdminSpeechProfileSchema,
  AdminListSpeechProfilesResponseSchema,
  ListSpeechProfilesResponseSchema,
} from './gen/postpilot/v1/speech_profile_pb'
export type {
  SpeechProfileChoice as ProtoSpeechProfileChoice,
  AdminSpeechProfile as ProtoAdminSpeechProfile,
  AdminListSpeechProfilesResponse as ProtoSpeechAdminBrowse,
  SpeechAccountTariff as ProtoSpeechAccountTariff,
} from './gen/postpilot/v1/speech_profile_pb'
export {
  ApplyCatalogDocumentResponseSchema,
  CatalogEntrySchema,
  ExportCatalogDocumentResponseSchema,
  ListCatalogResponseSchema,
  PreviewCatalogDocumentResponseSchema,
  SetModelPurposeResponseSchema,
  UpdateModelResponseSchema,
} from './gen/postpilot/v1/model_catalog_pb'
export type {
  CatalogEntry as ProtoCatalogEntry,
  ListCatalogResponse as ProtoListCatalogResponse,
  ApplyCatalogDocumentResponse as ProtoApplyCatalogDocumentResponse,
  PreviewCatalogDocumentResponse as ProtoPreviewCatalogDocumentResponse,
} from './gen/postpilot/v1/model_catalog_pb'
export {
  ProviderService,
  Stage,
  SelectionSlot,
  PostCreditsBasis,
} from './gen/postpilot/v1/provider_pb'
export { postCreditsBasisName, type PostCreditsBasisName } from './post-credits'
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
  SaveRecommendationSetResponseSchema,
  DeleteRecommendationSetResponseSchema,
  MoveRecommendationSetResponseSchema,
} from './gen/postpilot/v1/provider_pb'
export { VoiceService } from './gen/postpilot/v1/voice_pb'
export {
  AddVoiceSampleResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  VoiceProfileSchema,
  VoiceSampleSchema,
  VoiceSchema,
  ListVoicesResponseSchema,
  CreateVoiceResponseSchema,
  RenameVoiceResponseSchema,
  SetDefaultVoiceResponseSchema,
  DeleteVoiceResponseSchema,
  RestoreVoiceResponseSchema,
  VoiceSampleKind,
  VoicePromptPart,
  VoicePromptSchema,
  VoiceReadinessSchema,
  ListVoicePromptsResponseSchema,
  GetVoiceSampleResponseSchema,
  CreateVoicePhotoUploadResponseSchema,
  AnswerVoicePromptResponseSchema,
  AnalyzeVoiceResponseSchema,
  VoiceAiField,
  VoiceNoticeKind,
  VoiceNoticeSchema,
  VoiceAnalysisSchema,
  VoiceFingerprintSchema,
  RestorePreviousVoiceAnalysisResponseSchema,
  FingerprintItem as ProtoFingerprintItem,
  FingerprintFacetUnit as ProtoFingerprintFacetUnit,
  FingerprintItemComparisonSchema,
  GetPostFingerprintResponseSchema,
  VoiceCheckStatus as ProtoVoiceCheckStatus,
  VoiceCheckSchema,
  ListVoiceChecksResponseSchema,
  StartVoiceCheckResponseSchema,
  RetryVoiceCheckResponseSchema,
} from './gen/postpilot/v1/voice_pb'
export type {
  GetVoiceProfileResponse,
  ListVoicesResponse,
  Voice as ProtoVoice,
  VoiceProfile as ProtoVoiceProfile,
  VoiceSample as ProtoVoiceSample,
  VoicePrompt as ProtoVoicePrompt,
  VoiceReadiness as ProtoVoiceReadiness,
  VoiceAnalysis as ProtoVoiceAnalysis,
  VoiceExample as ProtoVoiceExample,
  VoiceFingerprint as ProtoVoiceFingerprint,
  VoiceNotice as ProtoVoiceNotice,
  FingerprintItemComparison as ProtoFingerprintItemComparison,
  FingerprintFacetValue as ProtoFingerprintFacetValue,
  GetPostFingerprintResponse,
  VoiceCheck as ProtoVoiceCheck,
} from './gen/postpilot/v1/voice_pb'
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
  ExperimentSource,
  ExperimentStatus,
  DisplaySide,
  CandidateStatus,
  ExperimentOutcome,
  ExperimentOrigin,
  LeaderboardScope,
  LeaderboardWindow,
  VerdictBadge,
  CostSource,
  LeaderboardEntrySchema,
  ListExperimentsResponseSchema,
  ModelExperimentSchema,
  StartExperimentResponseSchema,
} from './gen/postpilot/v1/model_experiment_pb'
export {
  GuidelineService,
  GuidelineSchema,
  GuidelineTemplateRefSchema,
  GuidelineScope as ProtoGuidelineScope,
  ListGuidelinesResponseSchema,
  CreateGuidelineResponseSchema,
  UpdateGuidelineResponseSchema,
  DeleteGuidelineResponseSchema,
  GuidelineCandidateSchema,
  ListGuidelineCandidatesResponseSchema,
  DismissGuidelineCandidateResponseSchema,
  DefaultGuidelineSchema,
  DefaultGuidelineCopySchema,
  GuidelineKind as ProtoGuidelineKind,
  SetDefaultGuidelineEnabledResponseSchema,
} from './gen/postpilot/v1/guideline_pb'
export type {
  Guideline as ProtoGuideline,
  GuidelineTemplateRef as ProtoGuidelineTemplateRef,
  GuidelineCandidate as ProtoGuidelineCandidate,
  DefaultGuideline as ProtoDefaultGuideline,
} from './gen/postpilot/v1/guideline_pb'
export {
  MemoryService,
  MemorySchema,
  MemoryKind as ProtoMemoryKind,
  MemoryCandidateSchema,
  ListMemoriesResponseSchema,
  CreateMemoryResponseSchema,
  UpdateMemoryResponseSchema,
  DeleteMemoryResponseSchema,
  StartMemoryExtractionResponseSchema,
  GetMemoryExtractionResponseSchema,
  ResolveMemoryExtractionResponseSchema,
} from './gen/postpilot/v1/memory_pb'
export type {
  Memory as ProtoMemory,
  MemoryCandidate as ProtoMemoryCandidate,
} from './gen/postpilot/v1/memory_pb'
export {
  QualityService,
  QualityMetric as ProtoQualityMetric,
  QualityVerdict as ProtoQualityVerdict,
  GetPostMeasurementResponseSchema,
  GetAccountQualityResponseSchema,
  QualityReadingSchema,
  QualityTitleSaturationSchema,
  QualityCrossPostPhrasesSchema,
  QualityInPostRepetitionSchema,
  QualityCompositionSchema,
} from './gen/postpilot/v1/quality_pb'
export type {
  QualityReading as ProtoQualityReading,
  GetPostMeasurementResponse as ProtoPostMeasurement,
  GetAccountQualityResponse as ProtoAccountQuality,
} from './gen/postpilot/v1/quality_pb'
export {
  TemplateService,
  TemplateSchema,
  TemplateRefSchema,
  ListTemplatesResponseSchema,
  CreateTemplateResponseSchema,
  UpdateTemplateResponseSchema,
  DeleteTemplateResponseSchema,
  GetFormatGuideResponseSchema,
  StartTemplateRequestResponseSchema,
  GetTemplateRequestResultResponseSchema,
  CancelTemplateRequestResponseSchema,
  EstimateTemplateRequestResponseSchema,
} from './gen/postpilot/v1/template_pb'
export type {
  Template as ProtoTemplate,
  TemplateRef as ProtoTemplateRef,
} from './gen/postpilot/v1/template_pb'
export type {
  ModelExperiment as ProtoModelExperiment,
  ExperimentCandidate as ProtoExperimentCandidate,
  LeaderboardEntry as ProtoLeaderboardEntry,
} from './gen/postpilot/v1/model_experiment_pb'
export {
  SpokenVoiceService,
  SpokenDraftSchema,
  SpokenCandidateSchema,
  SpokenVoiceSchema,
  SpokenProfileSnapshotSchema,
  SpokenSampleAccessResponseSchema,
} from './gen/postpilot/v1/spoken_voice_pb'
export type {
  SpokenDraft as ProtoSpokenDraft,
  SpokenVoice as ProtoSpokenVoice,
  SpokenProfileSnapshot as ProtoSpokenProfileSnapshot,
} from './gen/postpilot/v1/spoken_voice_pb'
export {
  SpokenVoiceGenerationService,
  SpokenOperationSchema,
  SpokenOperationResponseSchema,
  SpokenWorkQuoteSchema,
} from './gen/postpilot/v1/spoken_voice_generation_pb'
export type { SpokenOperation as ProtoSpokenOperation } from './gen/postpilot/v1/spoken_voice_generation_pb'

export { ClipSpeechService } from './gen/postpilot/v1/clip_speech_pb'

export { VoiceOrigin as ProtoVoiceOrigin } from './gen/postpilot/v1/voice_pb'
export { WritingVoiceCandidateService } from './gen/postpilot/v1/writing_voice_candidate_pb'

export {
  ConfigurationAuthoringService,
  ConfigurationKind as ProtoConfigurationKind,
  AuthoringMode as ProtoAuthoringMode,
} from './gen/postpilot/v1/configuration_authoring_pb'

// Frozen writing-test transport contracts. Domain UI imports these only through entity adapters.
export * from './gen/postpilot/v1/writing_test_pb'
export {
  AuthoringDraftState as ProtoAuthoringDraftState,
  PatchAuthoringDraftRequestSchema,
  ResetAuthoringChatRequestSchema,
  ResetAuthoringBaselineRequestSchema,
  ListAuthoringSummariesRequestSchema,
  ListAuthoringSummariesResponseSchema,
} from './gen/postpilot/v1/configuration_authoring_pb'
export {
  UpdateVoiceSampleRequestSchema,
  UpdateVoiceSampleResponseSchema,
} from './gen/postpilot/v1/voice_pb'
export type { ClipAnalysisPreparationResponse } from './gen/postpilot/v1/clip_source_pb'

export { ClipAnalysisPreparationResponseSchema } from './gen/postpilot/v1/clip_source_pb'

// Origin/request inspection contracts; reads are registered by their owning tasks.
export * from './gen/postpilot/v1/semantic_origin_pb'
export * from './gen/postpilot/v1/request_inspection_pb'
export {
  semanticOriginCategoryFromProto,
  originReviewStateFromProto,
  originFieldKindFromProto,
} from './semantic-origin'
export { requestInspectionFromProto } from './request-inspection'
export type { RequestInspectionView, RequestInspectionStatus } from './request-inspection'
