import type { Voice } from '@/entities/voice'

/** The two creations that offer what they are missing: a post (POST-121) and a clip (CLIP-223). */
export type CreationOfferKind = 'post' | 'clip'

/** How the offer gets the account a voice: a new one, or the one already being made — a second
 *  voice would leave the first one half made (POST-121). */
export type VoiceOffer = { action: 'create' } | { action: 'continue'; voiceId: string }

export interface CreationOfferMissing {
  voice?: VoiceOffer
  template: boolean
}

/** What a new post lacks: a made voice, a template, or both — undefined when it lacks neither. */
export function postOfferMissing(
  active: readonly Voice[],
  templateCount: number,
): CreationOfferMissing | undefined {
  const voice = active.some((candidate) => candidate.made) ? undefined : voiceOffer(active)
  const template = templateCount === 0
  return voice || template ? { voice, template } : undefined
}

/** What a new clip lacks. Never a voice: a clip takes no writing voice (CLIP-223). */
export function clipOfferMissing(templateCount: number): CreationOfferMissing | undefined {
  return templateCount === 0 ? { template: true } : undefined
}

function voiceOffer(active: readonly Voice[]): VoiceOffer {
  // ISO timestamps order as strings; the newest is the one the owner last started.
  const making = active
    .filter((candidate) => !candidate.made)
    .reduce<Voice | undefined>(
      (newest, candidate) =>
        !newest || candidate.createdAt > newest.createdAt ? candidate : newest,
      undefined,
    )
  return making ? { action: 'continue', voiceId: making.id } : { action: 'create' }
}
