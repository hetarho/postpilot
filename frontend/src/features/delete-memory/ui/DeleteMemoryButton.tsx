import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { useDeleteMemoryCall } from '@/entities/memory'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

/** Deletes a memory after the sheet says the one thing that matters: there is no undo (MEM-30).
 *  An icon at the end of the row's text line; its failure message takes a line of its own below
 *  (`w-full` in the row's wrapping flex line).
 *
 *  Nothing references a memory — a generation froze the TEXTS it selected at enqueue (MEM-19) —
 *  so there is no count to name and nothing in flight changes. */
export function DeleteMemoryButton({ ownerId, memoryId }: { ownerId: string; memoryId: string }) {
  const { t } = useTranslation(['memories', 'common'])
  const remove = useDeleteMemoryCall(ownerId)
  const [confirming, setConfirming] = useState(false)

  const confirm = async () => {
    try {
      await remove.remove(memoryId)
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
        variant="ghost"
        size="icon"
        disabled={remove.isPending}
        onClick={() => setConfirming(true)}
        aria-label={t('delete.aria', { ns: 'memories' })}
        className="shrink-0"
      >
        <Trash2 className="size-4" aria-hidden />
      </Button>
      {remove.isError && <FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      <Dialog
        open={confirming}
        title={t('delete.title', { ns: 'memories' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('delete.description', { ns: 'memories' })}
      </Dialog>
    </>
  )
}
