/** The two memory bounds the client counts against, mirrored from the backend's
 *  `MEMORY_TEXT_MAX_CHARS` and `MEMORY_TAGS_MAX` for the live counters; the server stays
 *  authoritative and its refusal is what a user reads. The per-account cap is deliberately NOT
 *  mirrored — it is the server's guard, and the create sheet relays its message rather than
 *  predicting it (MEM-11). */
import { ENV_LIMIT_OVERRIDES, positiveIntEnv } from '@/shared/config'

export const MEMORY_TEXT_MAX_CHARS = positiveIntEnv(ENV_LIMIT_OVERRIDES.memoryTextMaxChars, 120)
export const MEMORY_TAGS_MAX = positiveIntEnv(ENV_LIMIT_OVERRIDES.memoryTagsMax, 5)
