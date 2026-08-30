import { Button, FieldMessage } from '@/shared/ui'
import { useSetDefaultVoice } from '../api/useSetDefaultVoice'

/** Makes this voice the one a new post starts in. Nothing else changes: no profile work, and the
 *  previous default keeps everything it learned. */
export function SetDefaultVoiceButton({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const setDefault = useSetDefaultVoice(ownerId)
  return (
    <>
      <Button
        variant="secondary"
        pending={setDefault.isPending}
        onClick={() => void setDefault.setDefault(voiceId).catch(() => undefined)}
      >
        기본으로 설정
      </Button>
      {setDefault.isError && (
        <FieldMessage className="w-full">{setDefault.errorMessage}</FieldMessage>
      )}
    </>
  )
}
