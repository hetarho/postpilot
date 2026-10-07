import { POST_TARGET_LENGTH_MAX } from './index'

/** Version and independent bounds for result-local annotations; content stays usable on failure. */
export const ORIGIN_REVIEW_VERSION = 1
export const ORIGIN_MAX_SOURCES = 128
export const ORIGIN_MAX_REFS_PER_SPAN = ORIGIN_MAX_SOURCES
export const ORIGIN_SOURCE_ID_MAX_SCALARS = 128
export const ORIGIN_SOURCE_TEXT_MAX_SCALARS = POST_TARGET_LENGTH_MAX
