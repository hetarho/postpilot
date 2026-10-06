import { useTranslation } from 'react-i18next'
import { Meter } from '@/shared/ui'
import { VOICE_INITIAL_QUESTION_COUNT } from '../config'
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
  const required = readiness.requiredQuestions || VOICE_INITIAL_QUESTION_COUNT
  const answered = Math.min(required, readiness.answeredQuestions ?? 0)
  const short = ready ? 0 : Math.max(required - answered, 0)
  const note =
    short > 0
      ? t('readiness.questionsMore', { count: short })
      : missing && !ready
        ? t('readiness.coverage', { parts: missing })
        : ''
  return (
    <Meter
      label={ready ? t('readiness.ready') : t('readiness.label')}
      value={Math.min(readiness.percent, 100)}
      max={100}
      valueText={ready ? '100%' : t('readiness.questionProgress', { answered, required })}
      note={note || undefined}
      className={className}
    />
  )
}
