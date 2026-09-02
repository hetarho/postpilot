import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ContentRevisionConflictError, type PostDraft } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Notice, Popover, Typography } from '@/shared/ui'
import { useFinalizePost } from '../api/useFinalizePost'
import type { VoiceLearning } from '../model/useVoiceLearning'

type FinalizeMode = 'finalize' | 'learn'

/** The boundary that ends the drafting loop, as ONE control at the top-right of 글 다듬기's
 *  revision row (`widgets/refine-dock`).
 *
 *  확정하기 opens the two ways out — 확정 and 확정하고 말투 학습 — in a popover, and in a bottom
 *  sheet below `sm:`. They used to stand as a full-width pair directly under the revision field,
 *  which put a committing action inside the surface for asking the AI to rewrite the draft: two
 *  rows of controls in one small bar, where the user could not tell which of them was the
 *  conversation and which one ended the step (owner decision 2026-09-02). One trigger says which
 *  is which, and the choice between the two — the only thing that ever needed both of them on
 *  screen at once — is made on a surface with room to explain the difference.
 *
 *  The panel IS the confirmation: each action carries the sentence describing what it does, so the
 *  modal that used to stand between the press and the run is gone. A finalize is reversible by the
 *  next content save, which returns the post to `review`.
 *
 *  Both actions carry the user to 글 완성, so finishing a step is one gesture rather than a press
 *  plus a tab change. 확정 records the revision and nothing else; 확정하고 말투 학습 additionally
 *  starts the learning run, which is reported on 글 완성 because that is where it lands. */
export function FinalizeActions({
  post,
  learning,
  beforeFinalize,
  onFinalized,
}: {
  post: PostDraft
  learning: VoiceLearning
  /** Flushes pending edits and resolves the exact revision to finalize. */
  beforeFinalize: () => Promise<bigint>
  /** Carries the post's title AS THE SERVER NOW HOLDS IT: 확정 copies the finalized content's
   *  title into `posts.title`, and the editor still has the old 가제 in state where the next
   *  autosave would write it straight back. */
  onFinalized: (title: string) => void
}) {
  const { t } = useTranslation('posts')
  const finalize = useFinalizePost()
  const [preparing, setPreparing] = useState<FinalizeMode | ''>('')
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  // The status is the state (policy/posts.md): a content save after a finalize returns the post
  // to `review`, so this comes back on its own when the user edits again.
  const finalized = post.status === 'finalized'

  const run = async (mode: FinalizeMode, close: () => void) => {
    if (mode === 'learn' && !learning.canLearn) return
    setPreparing(mode)
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
      setPreparing('')
      return
    }
    try {
      const written = await finalize.finalize(post.slug, revision)
      close()
      // A learning failure does not undo the finalize written immediately before it, and it is
      // reported on 글 완성 — so the step moves either way.
      if (mode === 'learn') await learning.learn(revision).catch(() => undefined)
      onFinalized(written.title)
    } catch {
      // The finalize itself failed. Its mutation renders the error inside the still-open panel,
      // and the step holds.
    } finally {
      setPreparing('')
    }
  }

  // The way onward, and nothing else. The success banner that stood here said what the status
  // badge at the top of the editor already says, on a bar the draft is read past, and `finalized`
  // is a standing STATE rather than an event — so nothing took it down, and the first changed
  // content save returns the post to `review` and brings the trigger below back on its own
  // (owner decision 2026-09-02). 글 완성 reports the finalize and the learning run that may have
  // followed it; this step only has to offer the road there.
  if (finalized) {
    return (
      <Button variant="secondary" className="shrink-0" onClick={() => onFinalized(post.title)}>
        {t('finalize.goFinish')}
      </Button>
    )
  }

  return (
    <Popover
      label={t('finalize.open')}
      triggerLabel={t('finalize.open')}
      // It is what ENDS the step, so it carries the step's CTA fill even though it opens a surface
      // rather than committing on the spot.
      triggerVariant="cta"
      // The revision row sits at the top of a docked bar: the panel opens upward, over the draft,
      // with its right edge pinned to the trigger at the end of that row.
      placement="above"
      align="end"
      phone="sheet"
      className="shrink-0"
    >
      {(close) => (
        <div className="grid gap-4">
          {/* Every reason and every failure renders ABOVE the action it explains, so the software
              keyboard hides at most a button and never why it is disabled (§8.3). */}
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
          {/* 확정 leads: it is available whenever the draft can be finalized, where 확정하고 말투
              학습 additionally needs an analyze model and a baseline to learn from. In a vertical
              list the always-available action is the one the thumb of a one-handed grip reaches
              first (owner decision 2026-09-02). */}
          <div className="grid gap-1">
            <Button
              variant="cta"
              className="w-full"
              disabled={!post.canFinalize || learning.active}
              pending={preparing === 'finalize'}
              onClick={() => void run('finalize', close)}
            >
              {t('finalize.action')}
            </Button>
            <Typography variant="meta" as="p">
              {t('finalize.optionOnlyHint')}
            </Typography>
          </div>
          <div className="grid gap-1">
            {learning.blocked ? (
              <Typography variant="body" role="status" className="text-content-secondary">
                {learning.blocked}
              </Typography>
            ) : (
              learning.needsAnalyzeModel && (
                <Typography variant="body" className="text-content-tertiary">
                  {t('finalize.analyzeHelp')}
                </Typography>
              )
            )}
            <Button
              variant="secondary"
              className="w-full"
              disabled={!post.canFinalize || !learning.canLearn}
              pending={preparing === 'learn'}
              onClick={() => void run('learn', close)}
            >
              {t('finalize.actionLearn')}
            </Button>
            <Typography variant="meta" as="p">
              {t('finalize.optionLearnHint')}
            </Typography>
          </div>
        </div>
      )}
    </Popover>
  )
}
