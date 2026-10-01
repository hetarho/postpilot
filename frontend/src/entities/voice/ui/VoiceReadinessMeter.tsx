import { useTranslation } from 'react-i18next'
import { Meter } from '@/shared/ui'
import type { VoiceReadiness } from '../model/types'

/** The readiness meter of a voice not yet made (VOICE-32): `말투 학습에 필요한 정보 N% 확보`, how
 *  many sentences and which parts are still missing, and at 100% `이제 말투를 만들 수 있어요`. */
export function VoiceReadinessMeter({
  readiness,
  className,
}: {
  readiness: VoiceReadiness
  className?: string
}) {
  const { t } = useTranslation('voices')
  const ready = readiness.percent >= 100
  const missing = readiness.missingParts.map((part) => t(`readiness.part.${part}`)).join(' · ')
  const short = ready ? 0 : Math.max(readiness.needed - readiness.sentences, 0)
  const note = [
    short > 0 ? t('readiness.more', { sentences: short }) : '',
    missing ? t('readiness.missing', { parts: missing }) : '',
  ]
    .filter(Boolean)
    .join(' ')
  return (
    <Meter
      label={ready ? t('readiness.ready') : t('readiness.label')}
      value={Math.min(readiness.percent, 100)}
      max={100}
      valueText={ready ? '100%' : t('readiness.value', { percent: readiness.percent })}
      note={note || undefined}
      className={className}
    />
  )
}
