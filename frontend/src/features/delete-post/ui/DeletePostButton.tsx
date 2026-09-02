import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { displayTitle, useDeletePost, type PostDraft } from '@/entities/post'
import { Button, Dialog, FieldMessage } from '@/shared/ui'

/** Deletes the post the editor is showing. The server delete is hard — no trash, no restore —
 *  so the confirmation sheet is the whole protection and has to say what it destroys.
 *
 *  The trigger says 글 삭제하기 in words. It rides the editor's top row beside the status badge,
 *  and a bare `X` there read as "close the editor" rather than "destroy this post" (owner
 *  decision, 2026-09-02) — an icon alone is a guess, and this is the one control in the product
 *  whose guess cannot be undone. The visible label is therefore also its accessible name: an
 *  `aria-label` naming the post would no longer contain the visible text (WCAG 2.5.3), and the
 *  editor shows exactly one post, which the confirmation names again anyway. */
export function DeletePostButton({
  post,
  onDeleted,
}: {
  post: Pick<PostDraft, 'slug' | 'title'>
  /** Run after the server confirms the delete and BEFORE the navigation unmounts this. The
   *  autosave queues are what has to be stopped here, and they belong to sibling feature
   *  slices this one may not import (ARCHITECTURE §3.1), so the page supplies the call. */
  onDeleted?: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  const navigate = useNavigate()
  const remove = useDeletePost()
  const [confirming, setConfirming] = useState(false)
  const title = displayTitle(post)

  const confirm = async () => {
    try {
      await remove.remove(post.slug)
    } catch {
      // A refusal renders beside the trigger, and the dialog closes on failure too so the
      // message is not left behind the scrim. Only the DELETE is caught here: a navigation
      // that fails afterwards must not be reported as a refusal, because by then the post
      // really is gone.
      setConfirming(false)
      return
    }
    setConfirming(false)
    onDeleted?.()
    await navigate({ to: '/posts' })
  }

  return (
    <>
      <Button variant="danger" disabled={remove.isPending} onClick={() => setConfirming(true)}>
        {t('editor.delete.trigger', { ns: 'posts' })}
      </Button>
      {/* `w-full` inside the wrapping top row, so a refusal takes its own line under the trigger
          instead of squeezing the row it was pressed from (design-language §4.3 — feedback
          renders where the user is looking). */}
      {remove.isError && (
        <FieldMessage className="w-full">
          {remove.failure?.reason === 'POST_PUBLISHING'
            ? t('editor.delete.publishing', { ns: 'posts' })
            : remove.failure?.reason === 'POST_BUSY'
              ? t('editor.delete.busy', { ns: 'posts' })
              : remove.errorMessage}
        </FieldMessage>
      )}
      <Dialog
        open={confirming}
        title={t('editor.delete.title', { ns: 'posts' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('editor.delete.description', {
          ns: 'posts',
          title,
          interpolation: { escapeValue: false },
        })}
      </Dialog>
    </>
  )
}
