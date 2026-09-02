import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ContentRevisionConflictError, type PostDraft } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, Notice, Typography } from '@/shared/ui'
import { useFinalizePost } from '../api/useFinalizePost'
import type { VoiceLearning } from '../model/useVoiceLearning'

type FinalizeMode = 'finalize' | 'learn'

/** The boundary that ends the drafting loop, as the second row of 글 다듬기's dock
 *  (`widgets/refine-dock`).
 *
 *  It has no heading and no surface of its own: it used to sit at the end of a step whose draft is
 *  routinely thousands of pixels tall, so reaching either action meant scrolling to the end of the
 *  page (§4.3). Both actions carry the user to 글 완성, so finishing a step is one gesture rather
 *  than a press plus a tab change. 확정 records the revision and nothing else; 확정하고 말투 학습
 *  additionally starts the learning run, which is reported on 글 완성 because that is where it
 *  lands. */
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
  const [confirming, setConfirming] = useState<FinalizeMode | ''>('')
  const [preparing, setPreparing] = useState<FinalizeMode | ''>('')
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  // The status is the state (policy/posts.md): a content save after a finalize returns the post
  // to `review`, so this comes back on its own when the user edits again.
  const finalized = post.status === 'finalized'

  const run = async (mode: FinalizeMode) => {
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
      setConfirming('')
      // A learning failure does not undo the finalize written immediately before it, and it is
      // reported on 글 완성 — so the step moves either way.
      if (mode === 'learn') await learning.learn(revision).catch(() => undefined)
      onFinalized(written.title)
    } catch {
      // The finalize itself failed. Its mutation renders the error below, and the step holds.
    } finally {
      setPreparing('')
    }
  }

  return (
    <div className="grid gap-2">
      {/* Every reason renders ABOVE the pair it explains, so the software keyboard hides at most a
          button and never why it is disabled (§8.3). */}
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
      {finalized ? (
        // The way onward, and nothing else. The success banner that stood here said what the
        // status badge at the top of the editor already says, on a bar the draft is read past,
        // and `finalized` is a standing STATE rather than an event — so nothing took it down, and
        // the first changed content save returns the post to `review` and brings the pair below
        // back on its own (owner decision 2026-09-02). 글 완성 reports the finalize and the
        // learning run that may have followed it; this step only has to offer the road there.
        <Button variant="secondary" onClick={() => onFinalized(post.title)}>
          {t('finalize.goFinish')}
        </Button>
      ) : (
        <>
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
          {/* 확정 is the CTA and takes the right — it is what ENDS the step, it is available
              whenever the draft can be finalized, and 확정하고 말투 학습 additionally needs an
              analyze model and a baseline to learn from, so the always-available action is the
              one the thumb of a one-handed grip reaches first (owner decision 2026-09-02). The
              row still splits evenly rather than 3 : 7 like 글 생성's: 확정하고 말투 학습 is four
              words of Korean and a third of a 360px row would break it across three lines. */}
          <div className="grid grid-cols-2 gap-2">
            <Button
              variant="secondary"
              disabled={!post.canFinalize || !learning.canLearn}
              pending={preparing === 'learn'}
              onClick={() => setConfirming('learn')}
            >
              {t('finalize.actionLearn')}
            </Button>
            <Button
              variant="cta"
              disabled={!post.canFinalize || learning.active}
              pending={preparing === 'finalize'}
              onClick={() => setConfirming('finalize')}
            >
              {t('finalize.action')}
            </Button>
          </div>
        </>
      )}
      <Dialog
        open={Boolean(confirming)}
        title={t('finalize.confirmTitle')}
        confirmLabel={confirming === 'learn' ? t('finalize.confirmLearn') : t('finalize.action')}
        pending={Boolean(preparing)}
        onClose={() => setConfirming('')}
        onConfirm={() => confirming && void run(confirming)}
      >
        {confirming === 'learn'
          ? t('finalize.confirmLearnDescription')
          : t('finalize.confirmOnlyDescription')}
      </Dialog>
    </div>
  )
}
