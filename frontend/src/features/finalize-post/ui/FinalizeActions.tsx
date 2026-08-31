import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ContentRevisionConflictError, type PostDraft } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, Notice } from '@/shared/ui'
import { useFinalizePost } from '../api/useFinalizePost'
import type { VoiceLearning } from '../model/useVoiceLearning'

type FinalizeMode = 'finalize' | 'learn'

/** The bottom of 글 다듬기: the boundary that ends the drafting loop.
 *
 *  It sits at the end of the step whose work it closes — you finish reading the draft and the
 *  next thing under your thumb is the way out of it — and both actions carry the user to 글 완성,
 *  so finishing a step is one gesture rather than a click plus a tab change. 확정 records the
 *  revision and nothing else; 확정하고 말투 학습 additionally starts the learning run, which is
 *  reported on 글 완성 because that is where it lands. */
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
  onFinalized: () => void
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
      await finalize.finalize(post.slug, revision)
      setConfirming('')
      // A learning failure does not undo the finalize written immediately before it, and it is
      // reported on 글 완성 — so the step moves either way.
      if (mode === 'learn') await learning.learn(revision).catch(() => undefined)
      onFinalized()
    } catch {
      // The finalize itself failed. Its mutation renders the error below, and the step holds.
    } finally {
      setPreparing('')
    }
  }

  return (
    <section aria-labelledby="finalize-heading" className="mt-12">
      <h2 id="finalize-heading" className="text-lg font-semibold tracking-tight">
        {t('finalize.title')}
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        {finalized ? t('finalize.already') : t('finalize.description')}
      </p>
      {finalize.error && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={appFailureFromConnect(finalize.error)} />
        </Notice>
      )}
      {prepareFailure && (
        <Notice tone="danger" role="alert" className="mt-2">
          {prepareFailure === 'content-conflict' ? (
            t('edit.conflict')
          ) : (
            <AppFailureMessage failure={prepareFailure} />
          )}
        </Notice>
      )}
      {finalized ? (
        <>
          <Notice tone="success" role="status" className="mt-3">
            {t('finalize.success')}
          </Notice>
          <Button variant="secondary" className="mt-4" onClick={onFinalized}>
            {t('finalize.goFinish')}
          </Button>
        </>
      ) : (
        <>
          {learning.blocked ? (
            <p role="status" className="text-content-secondary mt-2 text-sm">
              {learning.blocked}
            </p>
          ) : (
            learning.needsAnalyzeModel && (
              <p className="text-content-tertiary mt-2 text-sm">{t('finalize.analyzeHelp')}</p>
            )
          )}
          <div className="mt-4 flex flex-wrap gap-2">
            <Button
              variant="secondary"
              disabled={!post.canFinalize || learning.active}
              pending={preparing === 'finalize'}
              onClick={() => setConfirming('finalize')}
            >
              {t('finalize.action')}
            </Button>
            <Button
              variant="cta"
              disabled={!post.canFinalize || !learning.canLearn}
              pending={preparing === 'learn'}
              onClick={() => setConfirming('learn')}
            >
              {t('finalize.actionLearn')}
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
    </section>
  )
}
