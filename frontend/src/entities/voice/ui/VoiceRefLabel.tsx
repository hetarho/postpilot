import { twMerge } from 'tailwind-merge'
import { voiceRefLabel, type VoiceRef } from '../model/types'

/** The voice a post is written in, as text. A tombstone says so in words — `삭제된 말투 · {name}` —
 *  rather than in colour, so the state survives a monochrome screen and a screen reader (§2.6).
 *  `min-w-0 truncate` because a voice name is user text in a flex row (§8.5). */
export function VoiceRefLabel({
  voice,
  className,
}: {
  voice: Pick<VoiceRef, 'name' | 'deleted'>
  className?: string
}) {
  return <span className={twMerge('min-w-0 truncate', className)}>{voiceRefLabel(voice)}</span>
}
