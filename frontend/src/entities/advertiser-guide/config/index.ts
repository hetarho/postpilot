import { ENV_LIMIT_OVERRIDES, positiveIntEnv } from '@/shared/config'

/** Mirrors of the backend's `internal/campaign` bounds (POST-118); the server is authoritative. */
export const ADVERTISER_GUIDE_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.advertiserGuideMaxChars,
  3000,
)
export const ADVERTISER_KEYWORD_MAX = positiveIntEnv(ENV_LIMIT_OVERRIDES.advertiserKeywordMax, 10)
/** Hashtags join a post's tags, so they follow its tag ceiling (GEN-46). */
export const ADVERTISER_HASHTAG_MAX = 30
export const ADVERTISER_ITEM_MAX_CHARS = 40
export const ADVERTISER_MIN_CHARS_RANGE = { min: 100, max: 10_000 } as const
export const ADVERTISER_MIN_PHOTOS_RANGE = { min: 1, max: 30 } as const
