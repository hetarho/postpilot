import { useState } from 'react'
import { detachWarning, type Purpose } from '@/entities/purpose'
import { Button, Dialog, FieldMessage } from '@/shared/ui'
import { useDeletePurpose } from '../api/useDeletePurpose'

/** Deletes a purpose after the sheet says exactly how many posts lose their assignment. The
 *  count comes from the server's projection, so the sentence and the write agree. */
export function DeletePurposeButton({
  ownerId,
  purpose,
}: {
  ownerId: string
  purpose: Pick<Purpose, 'id' | 'name' | 'postCount'>
}) {
  const remove = useDeletePurpose(ownerId)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.remove(purpose.id)
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
        aria-label={`${purpose.name} 삭제`}
      >
        삭제
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title="이 용도를 삭제할까요?"
        confirmLabel="삭제"
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        <span className="break-words">‘{purpose.name}’</span>을(를) 지웁니다.{' '}
        {detachWarning(purpose.postCount)} 이미 만들어진 글의 결과와 진행 중인 작업은 그대로예요.
      </Dialog>
    </>
  )
}
