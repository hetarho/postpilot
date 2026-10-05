import { useRef } from 'react'
import { useBlocker, useNavigate } from '@tanstack/react-router'
import { useCreateTemplate, useUpdateTemplate, type Template } from '@/entities/template'
import type { TemplateDraftState } from './useTemplateDraft'

/** What the save needs of the template request (TMPL-63): a running one locks the draft, is
 *  something to lose when leaving, and is cancelled by leaving. The request is its own feature,
 *  so only the shape it hands over is named here. */
export interface TemplateSaveRequest {
  running: boolean
  cancel: () => void
}

/** The one save of a template's screen (TMPL-25) and the guard on leaving it.
 *
 *  ONE DRAFT, ONE SAVE: `stored` decides between an update of all its fields in one call and a
 *  create that lands on the new template's own screen. What either wrote becomes the draft's
 *  clean baseline, and leaving with something to lose — an unsaved draft or a running request —
 *  asks first. */
export function useTemplateSave({
  ownerId,
  stored,
  draft,
  request,
}: {
  ownerId: string
  stored: Template | undefined
  draft: TemplateDraftState
  request: TemplateSaveRequest
}) {
  const navigate = useNavigate()
  const create = useCreateTemplate(ownerId)
  const update = useUpdateTemplate(ownerId, stored?.id ?? '')
  const pending = create.isPending || update.isPending
  // A running request locks the draft (TMPL-63): its answer lands in it.
  const locked = pending || request.running
  const errorMessage = create.errorMessage || update.errorMessage
  const failed = create.isError || update.isError
  const blocked = !draft.dirty || !draft.valid || locked

  // A REF, not state: the post-save redirect below runs in the same tick as the state update
  // that would clear `dirty`, and the blocker reads its render-time closure — so without this the
  // screen would intercept its own navigation and ask whether to discard a template it had just
  // created. It needs no reset: the route change remounts this component.
  const leavingAfterSave = useRef(false)
  // A running request is something to lose as much as an unsaved draft: leaving cancels it.
  const guard = () => request.running || (!leavingAfterSave.current && draft.dirty && !pending)

  // `enableBeforeUnload` is a FUNCTION, not the default `true`: the beforeunload path does not
  // consult `shouldBlockFn`, so leaving it alone would make the browser prompt on every reload of
  // a clean screen. A tab close still warns when there is something to lose — the browser's own
  // untranslatable prompt is a poor message, but losing an unsaved composition silently is worse.
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })

  const save = async () => {
    if (blocked) return
    const { trimmed } = draft
    try {
      if (stored) {
        // All three fields in one call. They are one decision now, and the server applies a
        // present field and leaves an absent one alone — so sending three is one transaction,
        // not a read-modify-write of anything the user did not touch on this screen.
        const updated = await update.saveAll(trimmed)
        draft.adoptSaved(updated.template)
        draft.markSaved()
        return
      }
      const created = await create.create(trimmed)
      const id = created.template?.id
      // The baseline moves BEFORE the navigation, or the blocker above would intercept the
      // screen's own redirect and ask whether to discard a template that was just created.
      draft.adoptSaved(created.template)
      if (!id) {
        // A create that answered without an id has nothing to navigate to. Staying put with the
        // draft intact is the honest outcome; navigating to an empty param would 404.
        draft.markSaved()
        return
      }
      leavingAfterSave.current = true
      // `replace`, so Back from the saved template goes to the list rather than to a `new`
      // screen that no longer describes anything.
      await navigate({ to: '/templates/$templateId', params: { templateId: id }, replace: true })
    } catch {
      // The mutation's message renders above the dock.
    }
  }

  return {
    pending,
    locked,
    /** 저장 is refused: nothing to save, a draft that cannot be saved, or something running. */
    blocked,
    failed,
    errorMessage,
    save,
    /** The leave guard's question, open while a navigation waits on the answer. */
    leave: {
      asking: blocker.status === 'blocked',
      stay: () => blocker.reset?.(),
      go: () => {
        // Leaving cancels the running request; it is not awaited (TMPL-63).
        if (request.running) request.cancel()
        blocker.proceed?.()
      },
    },
  }
}
