/** What the plan comparison's estimator starts from, and how far each slider goes.
 *
 *  The ceilings are the product's own: a post holds at most `UPLOAD_MAX_PHOTOS_PER_POST`
 *  photos and `UPLOAD_MAX_VIDEOS_PER_POST` clips, so an estimate outside them would price a
 *  post the product refuses to accept. The character range is editorial — a short review to a
 *  long one — and the default is the shape most posts in this product actually have. */
import { UPLOAD_MAX_PHOTOS_PER_POST, UPLOAD_MAX_VIDEOS_PER_POST } from '@/features/upload-photos'
import { CLIP_PROJECT_LIMITS } from '@/entities/clip-project'

export const PLAN_ESTIMATE_BOUNDS = {
  chars: { min: 200, max: 5_000, step: 100 },
  photos: { min: 0, max: UPLOAD_MAX_PHOTOS_PER_POST, step: 1 },
  videos: { min: 0, max: UPLOAD_MAX_VIDEOS_PER_POST, step: 1 },
} as const

export const PLAN_ESTIMATE_DEFAULTS = { chars: 1_000, photos: 5, videos: 0 } as const

/** Where the reader's own case is kept between visits. Namespaced like the theme's key. */
export const PLAN_ESTIMATE_STORAGE_KEY = 'postpilot.plan-estimate'

export const CLIP_ESTIMATE_BOUNDS = {
  sources: { min: 1, max: 20, step: 1 },
  seconds: { min: CLIP_PROJECT_LIMITS.minSeconds, max: CLIP_PROJECT_LIMITS.maxSeconds, step: 5 },
} as const
export const CLIP_ESTIMATE_DEFAULTS = { sources: 3, seconds: 30 } as const
export const CLIP_ESTIMATE_STORAGE_KEY = 'postpilot.plan-clip-estimate'
