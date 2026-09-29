import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  type PostDraft,
  ContentRevisionConflictError,
  isFinalizedOrLater,
  useFinalizePost,
} from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

/** The boundary that ends the drafting loop, as ONE control at the top-right of 글 다듬기's
 *  revision row (`widgets/refine-dock`).
 *
 *  확정하기 finalizes at once (POST-56): nothing stands between the press and the run, because a
 *  finalize is reversed by the next content save, which returns the post to `review`. It flushes
 *  every pending edit first, so it can never name a revision that omits one (POST-57), and it
 *  carries the user to 글 완성, so finishing a step is one gesture rather than a press plus a tab
 *  change.
 *
 *  The control lands wherever `children` puts it — inside the revision row's heading, a row too
 *  narrow to carry a sentence — so its failures render HERE, across the whole dock and above the
 *  row holding the control they explain: the software keyboard may hide a control but never its
 *  reason (THEME-31). */
export function FinalizeActions({
  post,
  beforeFinalize,
  onFinalized,
  children,
}: {
  post: PostDraft
  /** Flushes pending edits and resolves the exact revision to finalize. */
  beforeFinalize: () => Promise<bigint>
  /** Carries the post's title AS THE SERVER NOW HOLDS IT: 확정 copies the finalized content's
   *  title into `posts.title`, and the editor still has the old 가제 in state where the next
   *  autosave would write it straight back. */
  onFinalized: (title: string) => void
  /** Places the control: 확정하기, or the road onward once the post is past 확정. */
  children: (control: ReactNode) => ReactNode
}) {
  const { t } = useTranslation('posts')
  const finalize = useFinalizePost()
  const [preparing, setPreparing] = useState(false)
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  // The status is the state (POST-13): a content save after a finalize returns the post
  // to `review`, so this comes back on its own when the user edits again. A published post is past
  // 확정 and takes no finalize (POST-86), so it gets the same road onward.
  const finalized = isFinalizedOrLater(post)

  const run = async () => {
    setPreparing(true)
    setPrepareFailure(undefined)
    let revision: bigint
    try {
      revision = await beforeFinalize()
    } catch (cause) {
      setPrepareFailure(
        cause instanceof ContentRevisionConflictError
          ? 'content-conflict'
          : appFailureFromConnect(cause),
      )
      setPreparing(false)
      return
    }
    try {
      const written = await finalize.finalize(post.slug, revision)
      onFinalized(written.title)
    } catch {
      // The finalize itself failed. Its mutation renders the error above the row, and the step
      // holds.
    } finally {
      setPreparing(false)
    }
  }

  // The way onward, and nothing else. The success banner that stood here said what the status
  // badge at the top of the editor already says, on a bar the draft is read past, and `finalized`
  // is a standing STATE rather than an event — so nothing took it down, and the first changed
  // content save returns the post to `review` and brings 확정하기 back on its own (owner decision
  // 2026-09-02). 글 완성 reports the finalize; this step only has to offer the road there.
  const control = finalized ? (
    <Button variant="secondary" className="flex-1" onClick={() => onFinalized(post.title)}>
      {t('finalize.goFinish')}
    </Button>
  ) : (
    <Button
      variant="cta"
      // It FILLS the heading row's remaining width rather than shrinking to its two words: this is
      // the step's way out, and the row exists to hold it beside the field's name (THEME-23).
      className="flex-1"
      disabled={!post.canFinalize}
      pending={preparing}
      onClick={() => void run()}
    >
      {t('finalize.action')}
    </Button>
  )

  return (
    <>
      {finalize.error && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={appFailureFromConnect(finalize.error)} />
        </Notice>
      )}
      {prepareFailure && (
        <Notice tone="danger" role="alert">
          {prepareFailure === 'content-conflict' ? (
            t('edit.conflict')
          ) : (
            <AppFailureMessage failure={prepareFailure} />
          )}
        </Notice>
      )}
      {children(control)}
    </>
  )
}
