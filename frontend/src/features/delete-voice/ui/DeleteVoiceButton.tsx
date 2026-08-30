import { useState } from 'react'
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
        aria-label={`${voice.name} 삭제`}
      >
        삭제
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title="이 말투를 삭제할까요?"
        confirmLabel="삭제"
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        <span className="break-words">‘{voice.name}’</span>로 쓴 글과 학습 기록은 그대로 남아요. 그
        글에는 삭제된 말투로 표시되고, 복원하거나 다른 말투로 바꾸기 전에는 AI 생성·수정·학습을 할
        수 없어요. 새 글의 말투 목록에서는 바로 사라집니다.
      </Dialog>
    </>
  )
}
