import { create } from '@bufbuild/protobuf'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@connectrpc/connect-query'
import { useStageSelection } from '@/entities/model-catalog'
import type { VoiceProfile } from '@/entities/voice'
import { appFailureFromConnect, ModelRefSchema, VoiceValidationService } from '@/shared/api'
import { VOICE_VALIDATION_POST_COUNT } from '@/shared/config'
import {
  AppFailureMessage,
  Button,
  Checkbox,
  Dialog,
  Notice,
  Typography,
  typographyStyles,
} from '@/shared/ui'

/** Starts a validation of ONE voice's profile against that voice's own finalized sources; the
 *  request names the voice explicitly (spec/policy/voice.md). */
export function ValidateVoiceProfile({
  voiceId,
  profile,
  blocked = '',
}: {
  voiceId: string
  profile: VoiceProfile
  blocked?: string
}) {
  const { t } = useTranslation('voices')
  const analyze = useStageSelection('analyze')
  const write = useStageSelection('write')
  const mutation = useMutation(VoiceValidationService.method.startVoiceProfileValidation)
  const navigate = useNavigate()
  const [judge, setJudge] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const missing = Math.max(0, VOICE_VALIDATION_POST_COUNT - profile.finalizedSourceCount)
  const start = async () => {
    if (blocked || !analyze.selected || !write.selected) return
    const response = await mutation.mutateAsync({
      voiceId,
      analyzeModel: create(ModelRefSchema, analyze.selected),
      writeModel: create(ModelRefSchema, write.selected),
      judgeEnabled: judge,
    })
    setConfirming(false)
    if (response.validationId) {
      void navigate({
        to: '/voices/$voiceId/validations/$id',
        params: { voiceId, id: response.validationId },
      })
    }
  }
  return (
    <section aria-label={t('validate.title')}>
      <label
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 items-center gap-3',
        })}
      >
        <Checkbox
          checked={judge}
          disabled={Boolean(blocked)}
          onChange={(event) => setJudge(event.target.checked)}
        />
        {t('validate.judge')}
      </label>
      <Button
        variant="secondary"
        className="mt-3"
        disabled={Boolean(blocked) || !profile.canValidate || !analyze.selected || !write.selected}
        onClick={() => setConfirming(true)}
      >
        {t('validate.action')}
      </Button>
      {missing > 0 && (
        <Typography variant="body" className="text-content-tertiary mt-2">
          {t('validate.missing', { count: missing })}
        </Typography>
      )}
      {blocked && (
        <Notice tone="warning" role="status" className="mt-3">
          {blocked}
        </Notice>
      )}
      {mutation.error && (
        <Notice tone="danger" role="alert" className="mt-3">
          <AppFailureMessage failure={appFailureFromConnect(mutation.error)} />
        </Notice>
      )}
      <Dialog
        open={confirming}
        title={t('validate.confirmTitle')}
        confirmLabel={t('validate.confirm')}
        pending={mutation.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void start().catch(() => undefined)}
      >
        {judge ? t('validate.confirmJudgeDescription') : t('validate.confirmPlainDescription')}
      </Dialog>
    </section>
  )
}
