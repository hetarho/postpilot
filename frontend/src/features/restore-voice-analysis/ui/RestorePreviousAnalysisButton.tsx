import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRestorePreviousVoiceAnalysis } from '@/entities/voice'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

/** 이전 분석으로 되돌리기 (VOICE-30): the previous analysis becomes current and the replaced one is
 *  discarded, after the sheet says there is no redo. */
export function RestorePreviousAnalysisButton({
  ownerId,
  voiceId,
}: {
  ownerId: string
  voiceId: string
}) {
  const { t } = useTranslation(['voices', 'common'])
  const restore = useRestorePreviousVoiceAnalysis(ownerId, voiceId)
  const [confirming, setConfirming] = useState(false)
  const confirm = async () => {
    try {
      await restore.restore()
    } catch {
      // The mutation's message renders beside the button.
    } finally {
      setConfirming(false)
    }
  }
  return (
    <>
      <Button variant="ghost" disabled={restore.isPending} onClick={() => setConfirming(true)}>
        {t('undo.action', { ns: 'voices' })}
      </Button>
      {restore.isError && <FieldMessage className="w-full">{restore.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title={t('undo.title', { ns: 'voices' })}
        confirmLabel={t('undo.action', { ns: 'voices' })}
        pending={restore.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('undo.description', { ns: 'voices' })}
      </Dialog>
    </>
  )
}
