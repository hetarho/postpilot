import { twMerge } from 'tailwind-merge'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/shared/ui'
import { voiceRefLabel, type VoiceRef } from '../model/types'

/** The voice a post is written in, as text. A tombstone says so in words — `삭제된 말투 · {name}` —
 *  rather than in colour, so the state survives a monochrome screen and a screen reader (§2.6).
 *  `min-w-0 truncate` because a voice name is user text in a flex row (§8.5). */
export function VoiceRefLabel({
  voice,
  className,
}: {
  voice: Pick<VoiceRef, 'name' | 'deleted' | 'sourceLanguage'>
  className?: string
}) {
  const { t } = useTranslation(['voices', 'common'])
  return (
    <span className={twMerge('inline-flex min-w-0 items-center gap-2', className)}>
      <span className="truncate">{voiceRefLabel(voice)}</span>
      {voice.sourceLanguage && (
        <Badge>{t(`contentLanguage.${voice.sourceLanguage}`, { ns: 'common' })}</Badge>
      )}
    </span>
  )
}
