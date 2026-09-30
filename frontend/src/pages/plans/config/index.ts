/** How far each clip condition goes and where it starts. A post has no condition: its figure
 *  is recent real usage (QUOTA-64). The source ceiling and the finished-length range are the
 *  product's own. */
import { CLIP_PROJECT_LIMITS } from '@/entities/clip-project'

export const CLIP_ESTIMATE_BOUNDS = {
  sources: { min: 1, max: 20, step: 1 },
  seconds: { min: CLIP_PROJECT_LIMITS.minSeconds, max: CLIP_PROJECT_LIMITS.maxSeconds, step: 5 },
} as const
export const CLIP_ESTIMATE_DEFAULTS = { sources: 3, seconds: 30 } as const
export const CLIP_ESTIMATE_STORAGE_KEY = 'postpilot.plan-clip-estimate'
