import { twMerge } from 'tailwind-merge'
import { voiceRefLabel, type VoiceRef } from '../model/types'

/** The voice a post is written in, as text. A tombstone says so in words — `삭제된 말투 · {name}` —
 *  rather than in colour, so the state survives a monochrome screen and a screen reader (THEME-18).
 *  `min-w-0 truncate` because a voice name is user text in a flex row (THEME-32). */
export function VoiceRefLabel({
  voice,
  className,
}: {
  voice: Pick<VoiceRef, 'name' | 'deleted'>
  className?: string
}) {
  return (
    <span className={twMerge('inline-flex min-w-0 items-center gap-2', className)}>
      <span className="truncate">{voiceRefLabel(voice)}</span>
    </span>
  )
}
