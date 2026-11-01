import { useState } from 'react'
import { useClipTemplates } from '@/entities/clip-template'
import { useTemplates } from '@/entities/template'
import { useVoices } from '@/entities/voice'
import { readOfferDismissed, writeOfferDismissed } from './dismissal'
import {
  clipOfferMissing,
  postOfferMissing,
  type CreationOfferKind,
  type CreationOfferMissing,
} from './offer'

interface CreationOfferOptions {
  ownerId: string
  kind: CreationOfferKind
  /** The owner has just answered it: back from the offer's own wizard, or from first-use
   *  setup's finish (POST-121). */
  suppressed: boolean
}

/** Whether a new post or clip opens on the offer to make what the account is missing.
 *
 *  The answer is decided ONCE, when the directory reads first settle, and then held: the sheet
 *  opens before the owner has typed anything, and a refetch — the template directory re-reads on
 *  every mount (TMPL-31) — must neither reopen it nor change what it says while it is open. A
 *  read that failed is unknown, never empty, so it opens nothing (→AUTH-50). */
export function useCreationOffer({ ownerId, kind, suppressed }: CreationOfferOptions): {
  /** Every read this kind needs has finished, either way; the page waits for it so the sheet
   *  stands before the editor takes input. */
  settled: boolean
  open: boolean
  missing: CreationOfferMissing | undefined
  close: () => void
  dismiss: () => void
} {
  const post = kind === 'post'
  // Disabled reads for the other kind: a clip asks nothing about voices or post templates.
  const voices = useVoices(post ? ownerId : '')
  const templates = useTemplates(post ? ownerId : '')
  const clipTemplates = useClipTemplates(post ? '' : ownerId)
  const reads = post ? [voices, templates] : [clipTemplates]
  const readsSettled = !ownerId || reads.every((read) => !read.isPending)
  const [decision, setDecision] = useState<
    { open: boolean; missing: CreationOfferMissing | undefined } | undefined
  >()

  if (readsSettled && !decision) {
    const failed = !ownerId || reads.some((read) => read.isError)
    const missing = failed
      ? undefined
      : post
        ? postOfferMissing(voices.active, templates.templates.length)
        : clipOfferMissing(clipTemplates.templates.length)
    setDecision({
      open: !!missing && !suppressed && !readOfferDismissed(ownerId, kind),
      missing,
    })
  }

  return {
    // Held once decided as well. A failed read has no data, so the refetch the editor's own
    // pickers start on mount puts it back to pending; un-settling then would unmount the editor,
    // whose remount refetches again, without end.
    settled: readsSettled || !!decision,
    open: decision?.open ?? false,
    missing: decision?.missing,
    close: () => setDecision((current) => current && { ...current, open: false }),
    dismiss: () => {
      writeOfferDismissed(ownerId, kind)
      setDecision((current) => current && { ...current, open: false })
    },
  }
}
