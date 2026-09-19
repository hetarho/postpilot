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
import { i18n as finalizeClipClipsI18n } from '@/features/finalize-clip/config/i18n'
import { i18n as generateClipClipsI18n } from '@/features/generate-clip/config/i18n'
import { i18n as inspectClipObservationsClipsI18n } from '@/features/inspect-clip-observations/config/i18n'
import { i18n as renderClipBrowserClipsI18n } from '@/features/render-clip-browser/config/i18n'
import { i18n as reviseClipClipsI18n } from '@/features/revise-clip/config/i18n'
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
import { i18n as billingCheckoutI18n } from '@/pages/billing-checkout/config/i18n'
import { i18n as billingI18n } from '@/pages/billing/config/i18n'
import { i18n as cancelPublishI18n } from '@/features/cancel-publish/config/i18n'
import { i18n as candidateComparisonI18n } from '@/widgets/candidate-comparison/config/i18n'
import { i18n as configureModelPairI18n } from '@/features/configure-model-pair/config/i18n'
import { i18n as configurePublishingAgentI18n } from '@/features/configure-publishing-agent/config/i18n'
import { i18n as contactSheetI18n } from '@/widgets/contact-sheet/config/i18n'
import { i18n as createVoiceI18n } from '@/features/create-voice/config/i18n'
import { i18n as creditBadgeI18n } from '@/widgets/credit-badge/config/i18n'
import { i18n as deleteGuidelineI18n } from '@/features/delete-guideline/config/i18n'
import { i18n as deleteTemplateI18n } from '@/features/delete-template/config/i18n'
import { i18n as deleteVoiceI18n } from '@/features/delete-voice/config/i18n'
import { i18n as editGuidelineI18n } from '@/features/edit-guideline/config/i18n'
import { i18n as editVoiceProfileI18n } from '@/features/edit-voice-profile/config/i18n'
import { i18n as exportPanelI18n } from '@/widgets/export-panel/config/i18n'
import { i18n as finalizePostI18n } from '@/features/finalize-post/config/i18n'
import { i18n as giveVoiceFeedbackI18n } from '@/features/give-voice-feedback/config/i18n'
import { i18n as guidelineI18n } from '@/entities/guideline/config/i18n'
import { i18n as guidelinesI18n } from '@/pages/guidelines/config/i18n'
import { i18n as manageModelCatalogI18n } from '@/features/manage-model-catalog/config/i18n'
import { i18n as manageSubscriptionI18n } from '@/features/manage-subscription/config/i18n'
import { i18n as manageVoiceRulesI18n } from '@/features/manage-voice-rules/config/i18n'
import { i18n as manageVoiceSamplesI18n } from '@/features/manage-voice-samples/config/i18n'
import { i18n as modelCatalogI18n } from '@/entities/model-catalog/config/i18n'
import { i18n as modelExperimentI18n } from '@/pages/model-experiment/config/i18n'
import { i18n as modelLeaderboardI18n } from '@/widgets/model-leaderboard/config/i18n'
import { i18n as pairPublishingAgentI18n } from '@/features/pair-publishing-agent/config/i18n'
import { i18n as planI18n } from '@/entities/plan/config/i18n'
import { i18n as plansI18n } from '@/pages/plans/config/i18n'
import { i18n as postI18n } from '@/entities/post/config/i18n'
import { i18n as postsI18n } from '@/pages/posts/config/i18n'
import { i18n as publishJobI18n } from '@/entities/publish-job/config/i18n'
import { i18n as publishPanelI18n } from '@/widgets/publish-panel/config/i18n'
import { i18n as publishPostI18n } from '@/features/publish-post/config/i18n'
import { i18n as publishingAgentsI18n } from '@/pages/publishing-agents/config/i18n'
import { i18n as purchaseCreditsI18n } from '@/features/purchase-credits/config/i18n'
import { i18n as renameVoiceI18n } from '@/features/rename-voice/config/i18n'
import { i18n as retryPublishI18n } from '@/features/retry-publish/config/i18n'
import { i18n as reviewGuidelineCandidateI18n } from '@/features/review-guideline-candidate/config/i18n'
import { i18n as reviewModelExperimentI18n } from '@/features/review-model-experiment/config/i18n'
import { i18n as revokePublishingAgentI18n } from '@/features/revoke-publishing-agent/config/i18n'
import { i18n as selectPostTemplateI18n } from '@/features/select-post-template/config/i18n'
import { i18n as selectPostVoiceI18n } from '@/features/select-post-voice/config/i18n'
import { i18n as setDefaultVoiceI18n } from '@/features/set-default-voice/config/i18n'
import { i18n as subscriptionI18n } from '@/entities/subscription/config/i18n'
import { i18n as templateEntityI18n } from '@/entities/template/config/i18n'
import { i18n as templatePageI18n } from '@/pages/template/config/i18n'
import { i18n as templatesI18n } from '@/pages/templates/config/i18n'
import { i18n as uploadPhotosI18n } from '@/features/upload-photos/config/i18n'
import { i18n as validateVoiceProfileI18n } from '@/features/validate-voice-profile/config/i18n'
import { i18n as voiceEntityI18n } from '@/entities/voice/config/i18n'
import { i18n as voicePageI18n } from '@/pages/voice/config/i18n'
import { i18n as voiceRuleComparisonI18n } from '@/pages/voice-rule-comparison/config/i18n'
import { i18n as voiceValidationI18n } from '@/pages/voice-validation/config/i18n'
import { i18n as voicesI18n } from '@/pages/voices/config/i18n'
import { modelsI18n as selectModelModelsI18n } from '@/features/select-model/config/i18n'
import { plansI18n as selectModelPlansI18n } from '@/features/select-model/config/i18n'
import { postsI18n as editWithAiPostsI18n } from '@/features/edit-with-ai/config/i18n'

export const defaultNS = 'common' as const

export const RESOURCE_NAMESPACES = [
  'common',
  'auth',
  'nav',
  'posts',
  'voices',
  'templates',
  'guidelines',
  'models',
  'publishing',
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
export const FRAGMENTS: readonly I18nFragment[] = [
  cancelClipClipsI18n,
  clipDesignClipsI18n,
  clipPreviewClipsI18n,
  clipProjectClipsI18n,
  clipTemplateClipsI18n,
  clipWorkspaceClipsI18n,
  correctClipClipsI18n,
  deleteClipTemplateClipsI18n,
  editClipProjectClipsI18n,
  editClipTemplateClipsI18n,
  finalizeClipClipsI18n,
  generateClipClipsI18n,
  inspectClipObservationsClipsI18n,
  renderClipBrowserClipsI18n,
  reviseClipClipsI18n,
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
  configureModelPairI18n,
  manageModelCatalogI18n,
  modelCatalogI18n,
  modelExperimentI18n,
  modelLeaderboardI18n,
  reviewModelExperimentI18n,
  selectModelModelsI18n,
  creditBadgeI18n,
  planI18n,
  plansI18n,
  selectModelPlansI18n,
  candidateComparisonI18n,
  contactSheetI18n,
  editWithAiPostsI18n,
  exportPanelI18n,
  finalizePostI18n,
  postI18n,
  postsI18n,
  uploadPhotosI18n,
  cancelPublishI18n,
  configurePublishingAgentI18n,
  pairPublishingAgentI18n,
  publishJobI18n,
  publishPanelI18n,
  publishPostI18n,
  publishingAgentsI18n,
  retryPublishI18n,
  revokePublishingAgentI18n,
  deleteTemplateI18n,
  selectPostTemplateI18n,
  templateEntityI18n,
  templatePageI18n,
  templatesI18n,
  createVoiceI18n,
  deleteVoiceI18n,
  editVoiceProfileI18n,
  giveVoiceFeedbackI18n,
  manageVoiceRulesI18n,
  manageVoiceSamplesI18n,
  renameVoiceI18n,
  selectPostVoiceI18n,
  setDefaultVoiceI18n,
  validateVoiceProfileI18n,
  voiceEntityI18n,
  voicePageI18n,
  voiceRuleComparisonI18n,
  voiceValidationI18n,
  voicesI18n,
]

export const resources = {
  ko: {
    common: koCommon,
    auth: koAuth,
    nav: koNav,
    posts: {
      ...candidateComparisonI18n.ko,
      ...contactSheetI18n.ko,
      ...editWithAiPostsI18n.ko,
      ...exportPanelI18n.ko,
      ...finalizePostI18n.ko,
      ...postI18n.ko,
      ...postsI18n.ko,
      ...uploadPhotosI18n.ko,
    },
    voices: {
      ...createVoiceI18n.ko,
      ...deleteVoiceI18n.ko,
      ...editVoiceProfileI18n.ko,
      ...giveVoiceFeedbackI18n.ko,
      ...manageVoiceRulesI18n.ko,
      ...manageVoiceSamplesI18n.ko,
      ...renameVoiceI18n.ko,
      ...selectPostVoiceI18n.ko,
      ...setDefaultVoiceI18n.ko,
      ...validateVoiceProfileI18n.ko,
      ...voiceEntityI18n.ko,
      ...voicePageI18n.ko,
      ...voiceRuleComparisonI18n.ko,
      ...voiceValidationI18n.ko,
      ...voicesI18n.ko,
    },
    templates: {
      ...deleteTemplateI18n.ko,
      ...selectPostTemplateI18n.ko,
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
      ...reviewGuidelineCandidateI18n.ko,
    },
    models: {
      ...aiModelsI18n.ko,
      ...applyModelRecommendationI18n.ko,
      ...assignEstimatorComboI18n.ko,
      ...configureModelPairI18n.ko,
      ...manageModelCatalogI18n.ko,
      ...modelCatalogI18n.ko,
      ...modelExperimentI18n.ko,
      ...modelLeaderboardI18n.ko,
      ...reviewModelExperimentI18n.ko,
      ...selectModelModelsI18n.ko,
    },
    publishing: {
      ...cancelPublishI18n.ko,
      ...configurePublishingAgentI18n.ko,
      ...pairPublishingAgentI18n.ko,
      ...publishJobI18n.ko,
      ...publishPanelI18n.ko,
      ...publishPostI18n.ko,
      ...publishingAgentsI18n.ko,
      ...retryPublishI18n.ko,
      ...revokePublishingAgentI18n.ko,
    },
    errors: koErrors,
    marketing: koMarketing,
    plans: {
      ...creditBadgeI18n.ko,
      ...planI18n.ko,
      ...plansI18n.ko,
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
      ...deleteClipTemplateClipsI18n.ko,
      ...editClipProjectClipsI18n.ko,
      ...editClipTemplateClipsI18n.ko,
      ...finalizeClipClipsI18n.ko,
      ...generateClipClipsI18n.ko,
      ...inspectClipObservationsClipsI18n.ko,
      ...renderClipBrowserClipsI18n.ko,
      ...reviseClipClipsI18n.ko,
      ...uploadClipSourcesClipsI18n.ko,
      ...videoTemplatesClipsI18n.ko,
    },
  },
  en: {
    common: enCommon,
    auth: enAuth,
    nav: enNav,
    posts: {
      ...candidateComparisonI18n.en,
      ...contactSheetI18n.en,
      ...editWithAiPostsI18n.en,
      ...exportPanelI18n.en,
      ...finalizePostI18n.en,
      ...postI18n.en,
      ...postsI18n.en,
      ...uploadPhotosI18n.en,
    },
    voices: {
      ...createVoiceI18n.en,
      ...deleteVoiceI18n.en,
      ...editVoiceProfileI18n.en,
      ...giveVoiceFeedbackI18n.en,
      ...manageVoiceRulesI18n.en,
      ...manageVoiceSamplesI18n.en,
      ...renameVoiceI18n.en,
      ...selectPostVoiceI18n.en,
      ...setDefaultVoiceI18n.en,
      ...validateVoiceProfileI18n.en,
      ...voiceEntityI18n.en,
      ...voicePageI18n.en,
      ...voiceRuleComparisonI18n.en,
      ...voiceValidationI18n.en,
      ...voicesI18n.en,
    },
    templates: {
      ...deleteTemplateI18n.en,
      ...selectPostTemplateI18n.en,
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
      ...reviewGuidelineCandidateI18n.en,
    },
    models: {
      ...aiModelsI18n.en,
      ...applyModelRecommendationI18n.en,
      ...assignEstimatorComboI18n.en,
      ...configureModelPairI18n.en,
      ...manageModelCatalogI18n.en,
      ...modelCatalogI18n.en,
      ...modelExperimentI18n.en,
      ...modelLeaderboardI18n.en,
      ...reviewModelExperimentI18n.en,
      ...selectModelModelsI18n.en,
    },
    publishing: {
      ...cancelPublishI18n.en,
      ...configurePublishingAgentI18n.en,
      ...pairPublishingAgentI18n.en,
      ...publishJobI18n.en,
      ...publishPanelI18n.en,
      ...publishPostI18n.en,
      ...publishingAgentsI18n.en,
      ...retryPublishI18n.en,
      ...revokePublishingAgentI18n.en,
    },
    errors: enErrors,
    marketing: enMarketing,
    plans: {
      ...creditBadgeI18n.en,
      ...planI18n.en,
      ...plansI18n.en,
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
      ...deleteClipTemplateClipsI18n.en,
      ...editClipProjectClipsI18n.en,
      ...editClipTemplateClipsI18n.en,
      ...finalizeClipClipsI18n.en,
      ...generateClipClipsI18n.en,
      ...inspectClipObservationsClipsI18n.en,
      ...renderClipBrowserClipsI18n.en,
      ...reviseClipClipsI18n.en,
      ...uploadClipSourcesClipsI18n.en,
      ...videoTemplatesClipsI18n.en,
    },
  },
} as const
