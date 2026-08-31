import { useTranslation } from 'react-i18next'
import { FailureNotice, ProgressLine } from '@/entities/generation-job'
import { Button, FieldMessage, Notice } from '@/shared/ui'
import type { VoiceLearning } from '../model/useVoiceLearning'

/** 글 완성's own action, and the one place every learning outcome is reported.
 *
 *  The button is offered for a finalized revision that has not taught the voice anything yet; a
 *  revision already learned from keeps the button in place but disabled, so the completed run is
 *  visible as a state rather than as a control that has vanished. A run started by
 *  확정하고 말투 학습 on the previous step lands here too — the state is held above both panels. */
export function VoiceLearningPanel({
  learning,
  onBackToRefine,
}: {
  learning: VoiceLearning
  onBackToRefine: () => void
}) {
  const { t } = useTranslation('posts')
  // Said on screen rather than discovered through a refusal: a post edited after its finalize is
  // back in review and has to be confirmed again before it can teach anything.
  const finalized = learning.revisionFinalized

  return (
    <section aria-labelledby="learning-heading" className="mt-8">
      <h2 id="learning-heading" className="text-lg font-semibold tracking-tight">
        {t('learning.title')}
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        {t('learning.description')}
      </p>
      {learning.isError ? (
        <div className="mt-3">
          <FailureNotice message={t('learning.statusFailed')} onRetry={learning.refetch} />
        </div>
      ) : learning.retryable && learning.job ? (
        <div className="mt-3">
          <FailureNotice failure={learning.job.failure} onRetry={() => void learning.retry()} />
        </div>
      ) : learning.active && learning.job ? (
        <div className="mt-3">
          <ProgressLine job={learning.job} />
        </div>
      ) : learning.learned ? (
        <Notice tone="success" role="status" className="mt-3">
          {t('learning.learned')}
        </Notice>
      ) : finalized ? (
        <Notice tone="success" role="status" className="mt-3">
          {t('finalize.success')}
        </Notice>
      ) : null}
      {learning.errorMessage && (
        <FieldMessage className="mt-2">
          {t('learning.startFailedDetail', { error: learning.errorMessage })}
        </FieldMessage>
      )}
      {/* The voice gate outranks the finalize gate: telling someone to confirm a revision that
          could never be learned from either way would be a detour to the same refusal. */}
      {learning.blocked ? (
        <p role="status" className="text-content-secondary mt-2 text-sm">
          {learning.blocked}
        </p>
      ) : !finalized ? (
        <p role="status" className="text-content-secondary mt-2 text-sm">
          {t('learning.notFinalized')}
        </p>
      ) : (
        learning.needsAnalyzeModel && (
          <p className="text-content-tertiary mt-2 text-sm">{t('learning.needAnalyze')}</p>
        )
      )}
      <div className="mt-4 flex flex-wrap gap-2">
        <Button
          variant="cta"
          disabled={!learning.readyToLearn}
          pending={learning.pending}
          onClick={() => void learning.learn().catch(() => undefined)}
        >
          {t('learning.action')}
        </Button>
        {!finalized && !learning.blocked && (
          <Button variant="secondary" onClick={onBackToRefine}>
            {t('learning.goRefine')}
          </Button>
        )}
        {learning.noTextEdit && learning.learned && !learning.satisfied && (
          <Button
            variant="secondary"
            pending={learning.feedbackPending}
            onClick={() => void learning.satisfy()}
          >
            {t('learning.satisfied')}
          </Button>
        )}
      </div>
    </section>
  )
}
