import { writingTestI18n } from '@/features/writing-test/config/i18n'
import { voiceMaterialEditI18n } from '@/features/edit-voice-material/config/i18n'
import { navigationI18n } from '@/app/routes/navigation-i18n'
import { i18n as authoringI18n } from '@/features/ai-authoring/config/i18n'
import { i18n as voicePreparationI18n } from '@/features/prepare-writing-voice/config/i18n'
import { i18n as writingCandidatesI18n } from '@/entities/voice-candidate/config/i18n'
import { i18n as candidateFlowI18n } from '@/features/generate-writing-voices/config/i18n'
import { i18n as setupI18n } from '@/widgets/creation-setup/config/i18n'
import { i18n as homeI18n } from '@/pages/home/config/i18n'
import { i18n as libraryI18n } from '@/pages/library/config/i18n'
import { i18n as settingsI18n } from '@/pages/settings/config/i18n'
import { spokenVoiceResources } from '@/entities/spoken-voice/config/i18n'
import { createSpokenVoiceResources } from '@/features/create-spoken-voice/config/i18n'
import { manageSpokenVoiceResources } from '@/features/manage-spoken-voice/config/i18n'
import { spokenCreationResources } from '@/widgets/spoken-voice-creation/config/i18n'
import { spokenVoicesResources } from '@/pages/spoken-voices/config/i18n'
import { i18n as logInI18n } from '@/features/log-in/config/i18n'
import type { I18nFragment } from '@/shared/lib'
import { common as enCommon } from './en/common'
import { auth as enAuth } from './en/auth'
import { nav as enNav } from './en/nav'
import { errors as enErrors } from './en/errors'
import { marketing as enMarketing } from './en/marketing'
import { common as koCommon } from './ko/common'
import { auth as koAuth } from './ko/auth'
import { nav as koNav } from './ko/nav'
import { errors as koErrors } from './ko/errors'
import { marketing as koMarketing } from './ko/marketing'
import { i18n as cancelClipClipsI18n } from '@/features/cancel-clip/config/i18n'
import { i18n as clipDesignClipsI18n } from '@/entities/clip-design/config/i18n'
import { i18n as clipPreviewClipsI18n } from '@/entities/clip-preview/config/i18n'
import { i18n as clipProjectClipsI18n } from '@/entities/clip-project/config/i18n'
import { i18n as clipTemplateClipsI18n } from '@/entities/clip-template/config/i18n'
import { i18n as clipWorkspaceClipsI18n } from '@/widgets/clip-workspace/config/i18n'
import { i18n as correctClipClipsI18n } from '@/features/correct-clip/config/i18n'
import { i18n as deleteClipTemplateClipsI18n } from '@/features/delete-clip-template/config/i18n'
import { i18n as editClipProjectClipsI18n } from '@/features/edit-clip-project/config/i18n'
import { i18n as editClipTemplateClipsI18n } from '@/features/edit-clip-template/config/i18n'
import { i18n as editClipStorylineClipsI18n } from '@/features/edit-clip-storyline/config/i18n'
import { i18n as editClipRegionsClipsI18n } from '@/features/edit-clip-regions/config/i18n'
import { i18n as requestClipStorylineClipsI18n } from '@/features/request-clip-storyline/config/i18n'
import { i18n as finalizeClipClipsI18n } from '@/features/finalize-clip/config/i18n'
import { i18n as generateClipClipsI18n } from '@/features/generate-clip/config/i18n'
import { i18n as inspectClipObservationsClipsI18n } from '@/features/inspect-clip-observations/config/i18n'
import { i18n as renderClipBrowserClipsI18n } from '@/features/render-clip-browser/config/i18n'
import {
  i18n as reviseClipClipsI18n,
  guidelinesI18n as reviseClipGuidelinesI18n,
} from '@/features/revise-clip/config/i18n'
import { i18n as uploadClipSourcesClipsI18n } from '@/features/upload-clip-sources/config/i18n'
import { i18n as videoTemplatesClipsI18n } from '@/pages/video-templates/config/i18n'
// The fragments are imported as MODULES rather than through their slices' public APIs, and
// `steiger.config.ts` allows it for this file alone: a fragment is a leaf (its only import is
// the type above), while a slice barrel drags that slice's whole module graph in. Pulling 60
// barrels into the i18n assembly would evaluate most of the app before `main.tsx` renders — and,
// in tests, before a test's own module mocks are installed.
import { guidelinesI18n as editWithAiGuidelinesI18n } from '@/features/edit-with-ai/config/i18n'
import { i18n as accountMenuI18n } from '@/widgets/account-menu/config/i18n'
import { i18n as aiModelsI18n } from '@/pages/ai-models/config/i18n'
import { i18n as applyModelRecommendationI18n } from '@/features/apply-model-recommendation/config/i18n'
import { i18n as assignEstimatorComboI18n } from '@/features/assign-estimator-combo/config/i18n'
import { i18n as manageRecommendationSetI18n } from '@/features/manage-recommendation-set/config/i18n'
import { i18n as billingCheckoutI18n } from '@/pages/billing-checkout/config/i18n'
import { i18n as billingI18n } from '@/pages/billing/config/i18n'
import { i18n as blogFieldI18n } from '@/entities/blog-field/config/i18n'
import { i18n as candidateComparisonI18n } from '@/widgets/candidate-comparison/config/i18n'
import { i18n as voiceFingerprintI18n } from '@/widgets/voice-fingerprint/config/i18n'
import { i18n as configureModelPairI18n } from '@/features/configure-model-pair/config/i18n'
import { i18n as contactSheetI18n } from '@/widgets/contact-sheet/config/i18n'
import { i18n as createVoiceI18n } from '@/features/create-voice/config/i18n'
import { i18n as creditBadgeI18n } from '@/widgets/credit-badge/config/i18n'
import { i18n as deleteGuidelineI18n } from '@/features/delete-guideline/config/i18n'
import { i18n as deleteTemplateI18n } from '@/features/delete-template/config/i18n'
import { i18n as deleteVoiceI18n } from '@/features/delete-voice/config/i18n'
import { i18n as editGuidelineI18n } from '@/features/edit-guideline/config/i18n'
import { i18n as restoreVoiceAnalysisI18n } from '@/features/restore-voice-analysis/config/i18n'
import { i18n as exportPanelI18n } from '@/widgets/export-panel/config/i18n'
import { i18n as finalizePostI18n } from '@/features/finalize-post/config/i18n'
import { i18n as editStorylineI18n } from '@/features/edit-storyline/config/i18n'
import { i18n as generatePostI18n } from '@/features/generate-post/config/i18n'
import { i18n as storylineSpaceI18n } from '@/widgets/storyline-space/config/i18n'
import { i18n as recordPublishedUrlI18n } from '@/features/record-published-url/config/i18n'
import { i18n as selectPostFieldI18n } from '@/features/select-post-field/config/i18n'
import { i18n as qualityI18n } from '@/entities/quality/config/i18n'
import { i18n as chooseQualityRulesI18n } from '@/features/choose-quality-rules/config/i18n'
import { i18n as guidelineI18n } from '@/entities/guideline/config/i18n'
import { i18n as guidelinesI18n } from '@/widgets/guideline-directory/config/i18n'
import { i18n as memoryEntityI18n } from '@/entities/memory/config/i18n'
import { i18n as extractMemoriesI18n } from '@/features/extract-memories/config/i18n'
import { i18n as usePostMemoriesI18n } from '@/features/use-post-memories/config/i18n'
import { i18n as createMemoryI18n } from '@/features/create-memory/config/i18n'
import { i18n as editMemoryI18n } from '@/features/edit-memory/config/i18n'
import { i18n as deleteMemoryI18n } from '@/features/delete-memory/config/i18n'
import { i18n as memoriesPageI18n } from '@/pages/memories/config/i18n'
import { i18n as manageModelCatalogI18n } from '@/features/manage-model-catalog/config/i18n'
import { i18n as manageSubscriptionI18n } from '@/features/manage-subscription/config/i18n'
import { i18n as manageVoiceSamplesI18n } from '@/features/manage-voice-samples/config/i18n'
import { i18n as pasteVoiceMaterialI18n } from '@/features/paste-voice-material/config/i18n'
import { i18n as answerVoicePromptI18n } from '@/features/answer-voice-prompt/config/i18n'
import { i18n as makeVoiceI18n } from '@/features/make-voice/config/i18n'
import { i18n as modelCatalogI18n } from '@/entities/model-catalog/config/i18n'
import { i18n as modelExperimentEntityI18n } from '@/entities/model-experiment/config/i18n'
import { i18n as modelExperimentI18n } from '@/pages/model-experiment/config/i18n'
import { i18n as modelLeaderboardI18n } from '@/widgets/model-leaderboard/config/i18n'
import { i18n as planI18n } from '@/entities/plan/config/i18n'
import { i18n as plansI18n } from '@/pages/plans/config/i18n'
import { gift as giftPageI18n } from '@/pages/gift/config/i18n'
import { i18n as redeemVoucherI18n } from '@/features/redeem-voucher/config/i18n'
import { vouchers as adminVouchersI18n } from '@/pages/admin/config/i18n'
import { i18n as issueVoucherI18n } from '@/features/issue-voucher/config/i18n'
import { i18n as revokeVoucherI18n } from '@/features/revoke-voucher/config/i18n'
import { i18n as voucherEntityI18n } from '@/entities/voucher/config/i18n'
import { i18n as postI18n } from '@/entities/post/config/i18n'
import { i18n as postsI18n } from '@/pages/posts/config/i18n'
import { i18n as purchaseCreditsI18n } from '@/features/purchase-credits/config/i18n'
import { i18n as renameVoiceI18n } from '@/features/rename-voice/config/i18n'
import { i18n as reviewGuidelineCandidateI18n } from '@/features/review-guideline-candidate/config/i18n'
import { i18n as reviewModelExperimentI18n } from '@/features/review-model-experiment/config/i18n'
import { i18n as selectPostTemplateI18n } from '@/features/select-post-template/config/i18n'
import { i18n as requestTemplateI18n } from '@/features/request-template/config/i18n'
import { i18n as selectPostVoiceI18n } from '@/features/select-post-voice/config/i18n'
import { i18n as setDefaultVoiceI18n } from '@/features/set-default-voice/config/i18n'
import { i18n as subscriptionI18n } from '@/entities/subscription/config/i18n'
import { i18n as templateEntityI18n } from '@/entities/template/config/i18n'
import { i18n as templatePageI18n } from '@/pages/template/config/i18n'
import { i18n as templatesI18n } from '@/pages/templates/config/i18n'
import { i18n as uploadPhotosI18n } from '@/features/upload-photos/config/i18n'
import { i18n as voiceEntityI18n } from '@/entities/voice/config/i18n'
import { i18n as voicePageI18n } from '@/pages/voice/config/i18n'
import { i18n as voicesI18n } from '@/pages/voices/config/i18n'
import { modelsI18n as selectModelModelsI18n } from '@/features/select-model/config/i18n'
import { plansI18n as selectModelPlansI18n } from '@/features/select-model/config/i18n'
import { postsI18n as editWithAiPostsI18n } from '@/features/edit-with-ai/config/i18n'

export const defaultNS = 'common' as const

export const RESOURCE_NAMESPACES = [
  'authoring',
  'voicePreparation',
  'creation',
  'spokenVoice',
  'createSpokenVoice',
  'manageSpokenVoice',
  'spokenCreation',
  'spokenVoices',
  'common',
  'auth',
  'nav',
  'writingTests',
  'voiceMaterialEdit',
  'posts',
  'voices',
  'templates',
  'guidelines',
  'memories',
  'models',
  'errors',
  'marketing',
  'plans',
  'billing',
  'clips',
] as const

/** Every slice fragment, for the checks `resources.test.ts` runs over this assembly: two
 *  fragments of one namespace may not claim the same key, and every fragment must name a real
 *  namespace. `resources` below is built by spreading the same fragments, so the typed key shape
 *  stays the literal one i18next derives its types from (`i18next.d.ts`).
 *
 *  This list is the one file a new slice with its own strings touches — a one-line import
 *  instead of an edit inside a 1,100-line namespace file (ARCH-16). */
import { i18n as clipDubbingI18n } from '@/features/regenerate-clip-speech/config/i18n'

export const FRAGMENTS: readonly I18nFragment[] = [
  navigationI18n,
  writingTestI18n,
  voiceMaterialEditI18n,
  voicePreparationI18n,
  authoringI18n,
  writingCandidatesI18n,
  candidateFlowI18n,
  setupI18n,
  homeI18n,
  libraryI18n,
  settingsI18n,
  { namespace: 'spokenVoice', ...spokenVoiceResources },
  { namespace: 'createSpokenVoice', ...createSpokenVoiceResources },
  { namespace: 'manageSpokenVoice', ...manageSpokenVoiceResources },
  { namespace: 'spokenCreation', ...spokenCreationResources },
  { namespace: 'spokenVoices', ...spokenVoicesResources },
  logInI18n,
  extractMemoriesI18n,
  usePostMemoriesI18n,
  createMemoryI18n,
  deleteMemoryI18n,
  editMemoryI18n,
  memoriesPageI18n,
  memoryEntityI18n,
  cancelClipClipsI18n,
  clipDesignClipsI18n,
  clipPreviewClipsI18n,
  clipProjectClipsI18n,
  clipTemplateClipsI18n,
  clipWorkspaceClipsI18n,
  correctClipClipsI18n,
  clipDubbingI18n,
  deleteClipTemplateClipsI18n,
  editClipProjectClipsI18n,
  editClipTemplateClipsI18n,
  editClipStorylineClipsI18n,
  editClipRegionsClipsI18n,
  requestClipStorylineClipsI18n,
  finalizeClipClipsI18n,
  generateClipClipsI18n,
  inspectClipObservationsClipsI18n,
  renderClipBrowserClipsI18n,
  reviseClipClipsI18n,
  reviseClipGuidelinesI18n,
  uploadClipSourcesClipsI18n,
  videoTemplatesClipsI18n,
  accountMenuI18n,
  billingCheckoutI18n,
  billingI18n,
  manageSubscriptionI18n,
  purchaseCreditsI18n,
  subscriptionI18n,
  deleteGuidelineI18n,
  editGuidelineI18n,
  editWithAiGuidelinesI18n,
  guidelineI18n,
  guidelinesI18n,
  reviewGuidelineCandidateI18n,
  aiModelsI18n,
  applyModelRecommendationI18n,
  assignEstimatorComboI18n,
  manageRecommendationSetI18n,
  configureModelPairI18n,
  manageModelCatalogI18n,
  modelCatalogI18n,
  modelExperimentEntityI18n,
  modelExperimentI18n,
  modelLeaderboardI18n,
  reviewModelExperimentI18n,
  selectModelModelsI18n,
  creditBadgeI18n,
  planI18n,
  plansI18n,
  giftPageI18n,
  redeemVoucherI18n,
  adminVouchersI18n,
  issueVoucherI18n,
  revokeVoucherI18n,
  voucherEntityI18n,
  selectModelPlansI18n,
  blogFieldI18n,
  candidateComparisonI18n,
  contactSheetI18n,
  editWithAiPostsI18n,
  exportPanelI18n,
  finalizePostI18n,
  editStorylineI18n,
  generatePostI18n,
  storylineSpaceI18n,
  recordPublishedUrlI18n,
  selectPostFieldI18n,
  qualityI18n,
  chooseQualityRulesI18n,
  postI18n,
  postsI18n,
  uploadPhotosI18n,
  deleteTemplateI18n,
  selectPostTemplateI18n,
  requestTemplateI18n,
  templateEntityI18n,
  templatePageI18n,
  templatesI18n,
  createVoiceI18n,
  deleteVoiceI18n,
  restoreVoiceAnalysisI18n,
  voiceFingerprintI18n,
  manageVoiceSamplesI18n,
  pasteVoiceMaterialI18n,
  answerVoicePromptI18n,
  makeVoiceI18n,
  renameVoiceI18n,
  selectPostVoiceI18n,
  setDefaultVoiceI18n,
  voiceEntityI18n,
  voicePageI18n,
  voicesI18n,
]

export const resources = {
  ko: {
    authoring: { ...authoringI18n.ko },
    voicePreparation: { ...voicePreparationI18n.ko },
    creation: { ...homeI18n.ko, ...libraryI18n.ko, ...settingsI18n.ko, ...setupI18n.ko },
    spokenVoice: spokenVoiceResources.ko,
    createSpokenVoice: createSpokenVoiceResources.ko,
    manageSpokenVoice: manageSpokenVoiceResources.ko,
    spokenCreation: spokenCreationResources.ko,
    spokenVoices: spokenVoicesResources.ko,
    common: koCommon,
    auth: { ...koAuth, ...logInI18n.ko },
    nav: { ...koNav, ...navigationI18n.ko },
    writingTests: writingTestI18n.ko,
    voiceMaterialEdit: voiceMaterialEditI18n.ko,
    posts: {
      ...blogFieldI18n.ko,
      ...candidateComparisonI18n.ko,
      ...contactSheetI18n.ko,
      ...extractMemoriesI18n.ko,
      ...usePostMemoriesI18n.ko,
      ...editWithAiPostsI18n.ko,
      ...exportPanelI18n.ko,
      ...finalizePostI18n.ko,
      ...editStorylineI18n.ko,
      ...generatePostI18n.ko,
      ...storylineSpaceI18n.ko,
      ...recordPublishedUrlI18n.ko,
      ...selectPostFieldI18n.ko,
      ...qualityI18n.ko,
      ...chooseQualityRulesI18n.ko,
      ...postI18n.ko,
      ...postsI18n.ko,
      ...uploadPhotosI18n.ko,
    },
    voices: {
      ...writingCandidatesI18n.ko,
      ...candidateFlowI18n.ko,
      ...createVoiceI18n.ko,
      ...deleteVoiceI18n.ko,
      ...restoreVoiceAnalysisI18n.ko,
      ...voiceFingerprintI18n.ko,
      ...manageVoiceSamplesI18n.ko,
      ...pasteVoiceMaterialI18n.ko,
      ...answerVoicePromptI18n.ko,
      ...makeVoiceI18n.ko,
      ...renameVoiceI18n.ko,
      ...selectPostVoiceI18n.ko,
      ...setDefaultVoiceI18n.ko,
      ...voiceEntityI18n.ko,
      ...voicePageI18n.ko,
      ...voicesI18n.ko,
    },
    templates: {
      ...deleteTemplateI18n.ko,
      ...selectPostTemplateI18n.ko,
      ...requestTemplateI18n.ko,
      ...templateEntityI18n.ko,
      ...templatePageI18n.ko,
      ...templatesI18n.ko,
    },
    guidelines: {
      ...deleteGuidelineI18n.ko,
      ...editGuidelineI18n.ko,
      ...editWithAiGuidelinesI18n.ko,
      ...guidelineI18n.ko,
      ...guidelinesI18n.ko,
      ...reviseClipGuidelinesI18n.ko,
      ...reviewGuidelineCandidateI18n.ko,
    },
    memories: {
      ...createMemoryI18n.ko,
      ...deleteMemoryI18n.ko,
      ...editMemoryI18n.ko,
      ...memoriesPageI18n.ko,
      ...memoryEntityI18n.ko,
    },
    models: {
      ...aiModelsI18n.ko,
      ...applyModelRecommendationI18n.ko,
      ...assignEstimatorComboI18n.ko,
      ...manageRecommendationSetI18n.ko,
      ...configureModelPairI18n.ko,
      ...manageModelCatalogI18n.ko,
      ...modelCatalogI18n.ko,
      ...modelExperimentEntityI18n.ko,
      ...modelExperimentI18n.ko,
      ...modelLeaderboardI18n.ko,
      ...reviewModelExperimentI18n.ko,
      ...selectModelModelsI18n.ko,
    },
    errors: koErrors,
    marketing: koMarketing,
    plans: {
      ...creditBadgeI18n.ko,
      ...planI18n.ko,
      ...plansI18n.ko,
      ...giftPageI18n.ko,
      ...redeemVoucherI18n.ko,
      ...adminVouchersI18n.ko,
      ...issueVoucherI18n.ko,
      ...revokeVoucherI18n.ko,
      ...voucherEntityI18n.ko,
      ...selectModelPlansI18n.ko,
    },
    billing: {
      ...accountMenuI18n.ko,
      ...billingCheckoutI18n.ko,
      ...billingI18n.ko,
      ...manageSubscriptionI18n.ko,
      ...purchaseCreditsI18n.ko,
      ...subscriptionI18n.ko,
    },
    clips: {
      ...cancelClipClipsI18n.ko,
      ...clipDesignClipsI18n.ko,
      ...clipPreviewClipsI18n.ko,
      ...clipProjectClipsI18n.ko,
      ...clipTemplateClipsI18n.ko,
      ...clipWorkspaceClipsI18n.ko,
      ...correctClipClipsI18n.ko,
      ...clipDubbingI18n.ko,
      ...deleteClipTemplateClipsI18n.ko,
      ...editClipProjectClipsI18n.ko,
      ...editClipTemplateClipsI18n.ko,
      ...finalizeClipClipsI18n.ko,
      ...generateClipClipsI18n.ko,
      ...inspectClipObservationsClipsI18n.ko,
      ...renderClipBrowserClipsI18n.ko,
      ...reviseClipClipsI18n.ko,
      ...editClipStorylineClipsI18n.ko,
      ...editClipRegionsClipsI18n.ko,
      ...requestClipStorylineClipsI18n.ko,
      ...uploadClipSourcesClipsI18n.ko,
      ...videoTemplatesClipsI18n.ko,
    },
  },
  en: {
    authoring: { ...authoringI18n.en },
    voicePreparation: { ...voicePreparationI18n.en },
    creation: { ...homeI18n.en, ...libraryI18n.en, ...settingsI18n.en, ...setupI18n.en },
    spokenVoice: spokenVoiceResources.en,
    createSpokenVoice: createSpokenVoiceResources.en,
    manageSpokenVoice: manageSpokenVoiceResources.en,
    spokenCreation: spokenCreationResources.en,
    spokenVoices: spokenVoicesResources.en,
    common: enCommon,
    auth: { ...enAuth, ...logInI18n.en },
    nav: { ...enNav, ...navigationI18n.en },
    writingTests: writingTestI18n.en,
    voiceMaterialEdit: voiceMaterialEditI18n.en,
    posts: {
      ...blogFieldI18n.en,
      ...candidateComparisonI18n.en,
      ...contactSheetI18n.en,
      ...extractMemoriesI18n.en,
      ...usePostMemoriesI18n.en,
      ...editWithAiPostsI18n.en,
      ...exportPanelI18n.en,
      ...finalizePostI18n.en,
      ...editStorylineI18n.en,
      ...generatePostI18n.en,
      ...storylineSpaceI18n.en,
      ...recordPublishedUrlI18n.en,
      ...selectPostFieldI18n.en,
      ...qualityI18n.en,
      ...chooseQualityRulesI18n.en,
      ...postI18n.en,
      ...postsI18n.en,
      ...uploadPhotosI18n.en,
    },
    voices: {
      ...writingCandidatesI18n.en,
      ...candidateFlowI18n.en,
      ...createVoiceI18n.en,
      ...deleteVoiceI18n.en,
      ...restoreVoiceAnalysisI18n.en,
      ...voiceFingerprintI18n.en,
      ...manageVoiceSamplesI18n.en,
      ...pasteVoiceMaterialI18n.en,
      ...answerVoicePromptI18n.en,
      ...makeVoiceI18n.en,
      ...renameVoiceI18n.en,
      ...selectPostVoiceI18n.en,
      ...setDefaultVoiceI18n.en,
      ...voiceEntityI18n.en,
      ...voicePageI18n.en,
      ...voicesI18n.en,
    },
    templates: {
      ...deleteTemplateI18n.en,
      ...selectPostTemplateI18n.en,
      ...requestTemplateI18n.en,
      ...templateEntityI18n.en,
      ...templatePageI18n.en,
      ...templatesI18n.en,
    },
    guidelines: {
      ...deleteGuidelineI18n.en,
      ...editGuidelineI18n.en,
      ...editWithAiGuidelinesI18n.en,
      ...guidelineI18n.en,
      ...guidelinesI18n.en,
      ...reviseClipGuidelinesI18n.en,
      ...reviewGuidelineCandidateI18n.en,
    },
    memories: {
      ...createMemoryI18n.en,
      ...deleteMemoryI18n.en,
      ...editMemoryI18n.en,
      ...memoriesPageI18n.en,
      ...memoryEntityI18n.en,
    },
    models: {
      ...aiModelsI18n.en,
      ...applyModelRecommendationI18n.en,
      ...assignEstimatorComboI18n.en,
      ...manageRecommendationSetI18n.en,
      ...configureModelPairI18n.en,
      ...manageModelCatalogI18n.en,
      ...modelCatalogI18n.en,
      ...modelExperimentEntityI18n.en,
      ...modelExperimentI18n.en,
      ...modelLeaderboardI18n.en,
      ...reviewModelExperimentI18n.en,
      ...selectModelModelsI18n.en,
    },
    errors: enErrors,
    marketing: enMarketing,
    plans: {
      ...creditBadgeI18n.en,
      ...planI18n.en,
      ...plansI18n.en,
      ...giftPageI18n.en,
      ...redeemVoucherI18n.en,
      ...adminVouchersI18n.en,
      ...issueVoucherI18n.en,
      ...revokeVoucherI18n.en,
      ...voucherEntityI18n.en,
      ...selectModelPlansI18n.en,
    },
    billing: {
      ...accountMenuI18n.en,
      ...billingCheckoutI18n.en,
      ...billingI18n.en,
      ...manageSubscriptionI18n.en,
      ...purchaseCreditsI18n.en,
      ...subscriptionI18n.en,
    },
    clips: {
      ...cancelClipClipsI18n.en,
      ...clipDesignClipsI18n.en,
      ...clipPreviewClipsI18n.en,
      ...clipProjectClipsI18n.en,
      ...clipTemplateClipsI18n.en,
      ...clipWorkspaceClipsI18n.en,
      ...correctClipClipsI18n.en,
      ...clipDubbingI18n.en,
      ...deleteClipTemplateClipsI18n.en,
      ...editClipProjectClipsI18n.en,
      ...editClipTemplateClipsI18n.en,
      ...finalizeClipClipsI18n.en,
      ...generateClipClipsI18n.en,
      ...inspectClipObservationsClipsI18n.en,
      ...renderClipBrowserClipsI18n.en,
      ...reviseClipClipsI18n.en,
      ...editClipStorylineClipsI18n.en,
      ...editClipRegionsClipsI18n.en,
      ...requestClipStorylineClipsI18n.en,
      ...uploadClipSourcesClipsI18n.en,
      ...videoTemplatesClipsI18n.en,
    },
  },
} as const
