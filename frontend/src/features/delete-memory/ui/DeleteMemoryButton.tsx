import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { useDeleteMemoryCall } from '@/entities/memory'
import { Button, ConfirmedAction, FieldMessage } from '@/shared/ui'

/** Deletes a memory after the sheet says the one thing that matters: there is no undo (MEM-30).
 *  An icon at the end of the row's text line; its failure message takes a line of its own below
 *  (`w-full` in the row's wrapping flex line).
 *
 *  Nothing references a memory — a generation froze the TEXTS it selected at enqueue (MEM-19) —
 *  so there is no count to name and nothing in flight changes. */
export function DeleteMemoryButton({
  ownerId,
  memoryId,
  onDeleted,
}: {
  ownerId: string
  memoryId: string
  /** Run once the server confirms the delete. The list's re-read is not awaited, so the row may
   *  still be on screen when this runs, or already gone with this button in it. */
  onDeleted?: () => void
}) {
  const { t } = useTranslation(['memories', 'common'])
  const remove = useDeleteMemoryCall(ownerId)

  return (
    <ConfirmedAction
      trigger={(open) => (
        <Button
          variant="ghost"
          size="icon"
          disabled={remove.isPending}
          onClick={open}
          aria-label={t('delete.aria', { ns: 'memories' })}
          className="shrink-0"
        >
          <Trash2 className="size-4" aria-hidden />
        </Button>
      )}
      failed={remove.isError}
      failure={<FieldMessage className="w-full">{remove.errorMessage}</FieldMessage>}
      title={t('delete.title', { ns: 'memories' })}
      confirmLabel={t('action.delete', { ns: 'common' })}
      pending={remove.isPending}
      run={() => remove.remove(memoryId)}
      onDone={onDeleted}
    >
      {t('delete.description', { ns: 'memories' })}
    </ConfirmedAction>
  )
}
