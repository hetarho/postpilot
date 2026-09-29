import { useTranslation } from 'react-i18next'
import { Button, FieldMessage } from '@/shared/ui'
import { useSetDefaultVoice, type Voice } from '@/entities/voice'

/** `기본으로 설정` on a made voice that is not the 기본, `기본 해제` on the 기본 (VOICE-12, VOICE-54).
 *  A voice not yet made, or a tombstone, offers neither: the server refuses both. */
export function SetDefaultVoiceButton({
  ownerId,
  voice,
}: {
  ownerId: string
  voice: Pick<Voice, 'id' | 'isDefault' | 'made' | 'deleted'>
}) {
  const { t } = useTranslation('voices')
  const setDefault = useSetDefaultVoice(ownerId)
  if (voice.deleted || !voice.made) return null
  return (
    <>
      <Button
        variant="secondary"
        pending={setDefault.isPending}
        onClick={() =>
          void (
            voice.isDefault ? setDefault.clearDefault() : setDefault.setDefault(voice.id)
          ).catch(() => undefined)
        }
      >
        {voice.isDefault ? t('setDefault.clear') : t('setDefault.action')}
      </Button>
      {setDefault.isError && (
        <FieldMessage className="w-full">{setDefault.errorMessage}</FieldMessage>
      )}
    </>
  )
}
