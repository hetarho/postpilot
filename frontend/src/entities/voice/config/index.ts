/** The voice context's own field ceilings (ARCH-21), mirrored from the voice
 *  context on the server so a field can say so before the round trip; the server
 *  stays authoritative. Counted in Unicode scalar values, like the backend, so a
 *  Hangul syllable is one character here too. */

export const VOICE_NAME_MAX_CHARS = 50

/** The optional 말투 설명 ceiling, mirrored from `VoiceDescriptionMaxChars` the same way. It is
 *  far below a sample's length on purpose: this field states the register the user wants, it
 *  does not demonstrate it. */
export const VOICE_DESCRIPTION_MAX_CHARS = 500
