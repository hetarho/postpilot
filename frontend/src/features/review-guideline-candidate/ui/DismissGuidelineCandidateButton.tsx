import { useTranslation } from 'react-i18next'
import { useDismissGuidelineCandidateCall } from '@/entities/guideline'
import { Button, FieldMessage } from '@/shared/ui'

/** 무시 — marks the candidate dismissed and takes it out of the review list.
 *
 *  No confirmation dialog, unlike deleting a guideline: nothing is destroyed. The row is kept
 *  precisely because it is what stops the same instruction from being recorded again, and the
 *  correction can still be written by hand at any time. */
export function DismissGuidelineCandidateButton({
  ownerId,
  candidateId,
  disabled = false,
}: {
  ownerId: string
  candidateId: string
  disabled?: boolean
}) {
  const { t } = useTranslation('guidelines')
  const dismiss = useDismissGuidelineCandidateCall(ownerId)

  const run = async () => {
    try {
      await dismiss.dismiss(candidateId)
    } catch {
      // The mutation's message renders beside the button.
    }
  }

  return (
    <>
      <Button
        variant="ghost"
        disabled={disabled || dismiss.isPending}
        pending={dismiss.isPending}
        onClick={() => void run()}
      >
        {t('candidate.dismiss')}
      </Button>
      {dismiss.isError && <FieldMessage className="w-full">{dismiss.errorMessage}</FieldMessage>}
    </>
  )
}
