import { useId } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStageSelection } from '@/entities/model-catalog'
import { useAnalyzeVoice, type VoiceProfile } from '@/entities/voice'
import { Button, FieldMessage, Typography, typographyStyles } from '@/shared/ui'

/** 말투 만들기 before the voice is made, 다시 분석 after (VOICE-23): one analyze_voice job on the
 *  account's active analyze selection. Disabled below 100% or without a usable selection, each
 *  saying why in place (VOICE-55). */
export function MakeVoiceButton({
  ownerId,
  voiceId,
  profile,
  onStarted,
}: {
  ownerId: string
  voiceId: string
  profile: Pick<VoiceProfile, 'made' | 'readiness' | 'activeJobId' | 'voice'>
  onStarted: (jobId: string) => void
}) {
  const { t } = useTranslation('voices')
  const reasonId = useId()
  const { selected, isPending: modelPending } = useStageSelection('analyze')
  const analyze = useAnalyzeVoice(ownerId, voiceId)
  const ready = profile.readiness.percent >= 100
  const noModel = !modelPending && !selected
  const reason = profile.voice.deleted
    ? t('make.deleted')
    : !ready
      ? t('make.notReady')
      : noModel
        ? t('make.noModel')
        : ''
  const disabled = reason !== '' || modelPending || profile.activeJobId !== '' || analyze.isPending

  const start = async () => {
    if (disabled || !selected) return
    try {
      const response = await analyze.analyze(selected)
      onStarted(response.jobId)
    } catch {
      // The mutation's message renders under the button.
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <Button
        variant="cta"
        disabled={disabled}
        pending={analyze.isPending}
        aria-describedby={reason ? reasonId : undefined}
        onClick={() => void start()}
      >
        {profile.made ? t('make.again') : t('make.first')}
      </Button>
      {reason && (
        <Typography variant="label" as="p" id={reasonId} className="min-w-0">
          {reason}{' '}
          {noModel && ready && !profile.voice.deleted && (
            <Link
              to="/ai-models"
              className={typographyStyles({
                variant: 'label',
                className: 'text-link-fg hover:text-link-fg-hover underline',
              })}
            >
              {t('make.chooseModel')}
            </Link>
          )}
        </Typography>
      )}
      {analyze.isError && <FieldMessage className="w-full">{analyze.errorMessage}</FieldMessage>}
    </div>
  )
}
