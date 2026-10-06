/** The voice context's own field ceilings (ARCH-21), mirrored from the voice
 *  context on the server so a field can say so before the round trip; the server
 *  stays authoritative. Counted in Unicode scalar values, like the backend, so a
 *  Hangul syllable is one character here too. */

export const VOICE_NAME_MAX_CHARS = 50

export const VOICE_INITIAL_QUESTION_COUNT = 10
