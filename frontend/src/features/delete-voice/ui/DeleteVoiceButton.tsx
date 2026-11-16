import { useTranslation } from 'react-i18next'
import type { Voice } from '@/entities/voice'
import { Button, ConfirmedAction, FieldMessage } from '@/shared/ui'
import { useDeleteVoice } from '@/entities/voice'

/** Soft-deletes a voice after the sheet explains what stays (VOICE-13). The 기본 and the last
 *  voice delete like any other; only a voice with running work refuses. */
export function DeleteVoiceButton({
  ownerId,
  voice,
}: {
  ownerId: string
  voice: Pick<Voice, 'id' | 'name'>
}) {
  const { t } = useTranslation(['voices', 'common'])
  const remove = useDeleteVoice(ownerId)

  return (
    <ConfirmedAction
      trigger={(open) => (
        <Button
          variant="danger"
          disabled={remove.isPending}
          onClick={open}
          aria-label={t('delete.aria', { ns: 'voices', name: voice.name })}
        >
          {t('action.delete', { ns: 'common' })}
        </Button>
      )}
      failed={remove.isError}
      failure={
        <FieldMessage className="w-full">
          {remove.failure?.reason === 'VOICE_BUSY'
            ? t('error.deleteBlockedDetail', {
                ns: 'voices',
                error: remove.errorMessage,
                interpolation: { escapeValue: false },
              })
            : remove.errorMessage}
        </FieldMessage>
      }
      title={t('delete.title', { ns: 'voices' })}
      confirmLabel={t('action.delete', { ns: 'common' })}
      pending={remove.isPending}
      run={() => remove.remove(voice.id)}
    >
      {t('delete.description', { ns: 'voices' })}
    </ConfirmedAction>
  )
}
