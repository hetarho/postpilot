import type { VoiceExperience, VoiceExperienceKindInfo, VoicePrompt } from './types'

const PLACEHOLDER = '{experience}'

/** A question's text with its experience named (VOICE-78): `{experience}` becomes the experience's
 *  name, or the kind's plain word when the name is empty or no experience is given. A situation
 *  carries no placeholder and reads unchanged. */
export function promptText(
  prompt: Pick<VoicePrompt, 'text' | 'experienceKind'>,
  kinds: readonly VoiceExperienceKindInfo[],
  experience?: Pick<VoiceExperience, 'name'>,
): string {
  if (!prompt.text.includes(PLACEHOLDER)) return prompt.text
  const name = experience?.name.trim() ?? ''
  const plain = kinds.find((info) => info.kind === prompt.experienceKind)?.plain ?? ''
  return prompt.text.split(PLACEHOLDER).join(name || plain)
}
