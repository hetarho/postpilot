/** The voice context's own field ceilings (ARCH-21), mirrored from the voice
 *  context on the server so a field can say so before the round trip; the server
 *  stays authoritative. Counted in Unicode scalar values, like the backend, so a
 *  Hangul syllable is one character here too. */

export const VOICE_NAME_MAX_CHARS = 50

export const VOICE_INITIAL_QUESTION_COUNT = 10

/** One experience asks one opening, three description and one closing question, so two of them
 *  make the first ten and cover every part (VOICE-78). Mirrors `VOICE_EXPERIENCE_REPLIES` on the
 *  server. */
export const VOICE_EXPERIENCE_PART_QUOTA = { opening: 1, description: 3, closing: 1 } as const
export const VOICE_EXPERIENCE_REPLIES = 5
/** The name an owner may give an experience (`n / 40자`). */
export const VOICE_EXPERIENCE_NAME_MAX_CHARS = 40
