import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Voice } from '@/entities/voice'
import { Button, Dialog, FieldMessage } from '@/shared/ui'
import { useDeleteVoice } from '../api/useDeleteVoice'

/** Soft-deletes a voice after the sheet explains what stays. Never offered for the default: the
 *  server refuses it, and a button that always fails is not a control. */
export function DeleteVoiceButton({
  ownerId,
  voice,
}: {
  ownerId: string
  voice: Pick<Voice, 'id' | 'name' | 'isDefault'>
}) {
  const { t } = useTranslation(['voices', 'common'])
  const remove = useDeleteVoice(ownerId)
  const [confirming, setConfirming] = useState(false)
  if (voice.isDefault) return null

  const confirm = async () => {
    try {
      await remove.remove(voice.id)
    } catch {
      // The mutation's message renders beside the button.
    } finally {
      // Closed on failure too, so the message is not left behind the scrim.
      setConfirming(false)
    }
  }

  return (
    <>
      <Button
        variant="danger"
        disabled={remove.isPending}
        onClick={() => setConfirming(true)}
        aria-label={t('delete.aria', { ns: 'voices', name: voice.name })}
      >
        {t('action.delete', { ns: 'common' })}
      </Button>
      {remove.isError && (
        <FieldMessage className="w-full">
          {remove.failure?.reason === 'VOICE_BUSY' ||
          remove.failure?.reason === 'VOICE_DEFAULT_DELETE_FORBIDDEN'
            ? t('error.deleteBlockedDetail', {
                ns: 'voices',
                error: remove.errorMessage,
                interpolation: { escapeValue: false },
              })
            : remove.errorMessage}
        </FieldMessage>
      )}
      <Dialog
        open={confirming}
        title={t('delete.title', { ns: 'voices' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('delete.description', { ns: 'voices', name: voice.name })}
      </Dialog>
    </>
  )
}
