export type { AdvertiserGuide, AdvertiserGuideItem } from './model/types'
export { EMPTY_GUIDE, copyGuide, guideLines, isEmptyGuide, sameGuide } from './model/types'
export {
  ADVERTISER_GUIDE_MAX_CHARS,
  ADVERTISER_MIN_CHARS_RANGE,
  ADVERTISER_MIN_PHOTOS_RANGE,
  ADVERTISER_HASHTAG_MAX,
  ADVERTISER_ITEM_MAX_CHARS,
  ADVERTISER_KEYWORD_MAX,
} from './config'
export { advertiserGuideFromProto, advertiserGuideToProto } from './api/mappers'
export { AdvertiserGuideFields } from './ui/AdvertiserGuideFields'
export {
  acceptRequirement,
  editSuggestion,
  keywordCoverage,
  suggestRequirements,
  suggestionAlreadyAdded,
  type RequirementSuggestion,
} from './lib/scan'
export { AdvertiserGuideSuggestions } from './ui/AdvertiserGuideSuggestions'
export { RequirementsTracker, type RequirementsCoverage } from './ui/RequirementsTracker'
