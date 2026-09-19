/** Writing-guideline (작문 지침) text ceiling, mirrored from `GUIDELINE_TEXT_MAX_CHARS` for the
 *  live counter; the server stays authoritative. The per-account cap is deliberately not
 *  mirrored — it is a prompt-size guard the backend owns, and the create form relays its
 *  refusal message instead of predicting it. */
import { ENV_LIMIT_OVERRIDES, positiveIntEnv } from '@/shared/config'
export const GUIDELINE_TEXT_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.guidelineTextMaxChars,
  300,
)
