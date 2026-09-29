import { useTranslation } from 'react-i18next'
import { useRetryVoiceCheck } from '@/entities/voice'
import { Button, FieldMessage } from '@/shared/ui'
import { useWriteSelection } from '../model/useWriteSelection'

/** A failed 검증's `다시 검증`: a new check on the same prompt against the current analysis, with
 *  the account's write selection (VOICE-44). */
export function RetryCheckButton({
  ownerId,
  voiceId,
  checkId,
  disabled = false,
  onStarted,
}: {
  ownerId: string
  voiceId: string
  checkId: string
  disabled?: boolean
  onStarted: (jobId: string) => void
}) {
  const { t } = useTranslation('voices')
  const write = useWriteSelection()
  const retry = useRetryVoiceCheck(ownerId, voiceId)
  const run = async () => {
    if (!write.selected) return
    try {
      const { jobId } = await retry.retry(checkId, write.selected)
      onStarted(jobId)
    } catch {
      // The mutation's message renders under the button.
    }
  }
  return (
    <div className="flex flex-col items-start gap-2">
      <Button
        variant="secondary"
        disabled={disabled || !write.selected}
        pending={retry.isPending}
        onClick={() => void run()}
      >
        {t('check.retry')}
      </Button>
      {retry.isError && <FieldMessage>{retry.errorMessage}</FieldMessage>}
    </div>
  )
}
