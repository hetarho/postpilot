import { Button, FieldMessage } from '@/shared/ui'
import { useRestoreVoice } from '../api/useRestoreVoice'

/** Brings a deleted voice back exactly as it was — no job, no profile change, not made default. */
export function RestoreVoiceButton({
  ownerId,
  voiceId,
  variant = 'secondary',
  className,
}: {
  ownerId: string
  voiceId: string
  variant?: 'secondary' | 'ghost'
  className?: string
}) {
  const restore = useRestoreVoice(ownerId)
  return (
    <>
      <Button
        variant={variant}
        pending={restore.isPending}
        onClick={() => void restore.restore(voiceId).catch(() => undefined)}
        className={className}
      >
        복원
      </Button>
      {restore.isError && <FieldMessage className="w-full">{restore.errorMessage}</FieldMessage>}
    </>
  )
}
