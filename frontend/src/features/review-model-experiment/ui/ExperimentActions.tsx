import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { type QueryKey, useQueryClient } from '@tanstack/react-query'
import type { ModelExperiment } from '@/entities/model-experiment'
import { needsExperimentReview, useExperimentActions } from '@/entities/model-experiment'
import { getPostQueryKey, listPostsQueryKey } from '@/entities/post'
import { getSelectionsQueryKey } from '@/entities/model-catalog'
import { useSession } from '@/entities/session'
import { useVoices, voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice'
import { AppFailureMessage, Button, Dialog, Notice } from '@/shared/ui'
import { hasExperimentActions } from '../model/experiment-actions'

export function ExperimentActions({
  experiment,
  activeCandidateId,
}: {
  experiment: ModelExperiment
  activeCandidateId: string
}) {
  const { t } = useTranslation('models')
  const transport = useTransport()
  const queryClient = useQueryClient()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { voices, isPending: voicesPending } = useVoices(ownerId)
  const frozenVoice = experiment.voiceId
    ? voices.find((voice) => voice.id === experiment.voiceId)
    : undefined
  // Applying/retrying can publish into the experiment's frozen voice. Keep account-scoped
  // decisions and model adoption available, but never let provider/profile/post work target a
  // tombstone (or an unverified route cache entry).
  const voiceWorkBlocked = Boolean(
    experiment.voiceId && (voicesPending || !frozenVoice || frozenVoice.deleted),
  )
  const refreshOwner = useCallback(async () => {
    const keys: QueryKey[] = [listPostsQueryKey(transport), getSelectionsQueryKey(transport)]
    if (experiment.postSlug) keys.push(getPostQueryKey(transport, experiment.postSlug))
    // An applied analyze winner publishes a new head for the experiment's frozen voice only.
    if (experiment.voiceId) {
      keys.push(
        voiceProfileQueryKey(transport, ownerId, experiment.voiceId),
        voiceVersionsQueryKey(transport, ownerId, experiment.voiceId),
      )
    }
    await Promise.all(keys.map((queryKey) => queryClient.invalidateQueries({ queryKey })))
  }, [experiment.postSlug, experiment.voiceId, ownerId, queryClient, transport])
  const actions = useExperimentActions(experiment.id, refreshOwner)
  const [confirmStyle, setConfirmStyle] = useState(false)
  // `useExperimentActions` reports one `isPending` for all six mutations, so the button the thumb
  // actually pressed has to be remembered here — otherwise the whole bar spins at once. Without a
  // pending state at all, a tap on a cellular connection only dropped the button to 50% opacity,
  // which is not feedback on a device with no hover, and the user fired the mutation twice (§6).
  const [pressed, setPressed] = useState('')
  const run = (name: string, action: () => Promise<unknown>) => {
    setPressed(name)
    void action().finally(() => setPressed(''))
  }
  const selected = experiment.candidates.find((candidate) => candidate.id === activeCandidateId)
  const survivor = experiment.status === 'partial' && selected?.status === 'succeeded'
  const canChoose = experiment.status === 'review' && selected?.status === 'succeeded'
  if (!hasExperimentActions(experiment)) return null
  return (
    <div className="grid gap-3">
      {/* Full-width targets on a phone: three Korean labels measure ~410px against the 296px the
          bar has at 360px, so a wrapping row became two right-aligned rows of ambiguous targets
          8px apart (§4.1). One row per action is the default; the write decision's two committing
          actions pair off into a single row of their own below, because three stacked rows of
          chrome hide the draft the decision is about. From `sm:` up everything collapses back
          into the desktop row. The CTA is the last child in every status (§4). */}
      <div className="grid gap-3 sm:flex sm:flex-wrap sm:justify-end">
        {(experiment.status === 'partial' || experiment.status === 'failed') && (
          <Button
            variant="secondary"
            disabled={actions.isPending || voiceWorkBlocked}
            pending={pressed === 'retry'}
            onClick={() => run('retry', actions.retry)}
          >
            {t('actions.retryFailed')}
          </Button>
        )}
        {/* `compact` on the way out: the dock stands over the very draft the decision is about,
            so on a phone this row is 36px of chrome instead of 44px. It stretches back to its
            siblings' height inside the `sm:` flex row. */}
        {needsExperimentReview(experiment.status) && (
          <Button
            variant="ghost"
            size="compact"
            disabled={actions.isPending}
            pending={pressed === 'dismiss'}
            onClick={() => run('dismiss', actions.dismiss)}
          >
            {t('actions.dismiss')}
          </Button>
        )}
        {experiment.applyFailure && (
          <Button
            variant="secondary"
            disabled={actions.isPending || voiceWorkBlocked}
            pending={pressed === 'apply'}
            onClick={() =>
              run('apply', () =>
                experiment.stage === 'write'
                  ? actions.decideWrite(experiment.winnerCandidateId, experiment.adoptionRequested)
                  : actions.apply(experiment.stage === 'analyze'),
              )
            }
          >
            {t('actions.retryApply')}
          </Button>
        )}
        {experiment.status === 'decided' && experiment.stage !== 'write' && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'adopt'}
            onClick={() => run('adopt', actions.adopt)}
          >
            {t('actions.useActive')}
          </Button>
        )}
        {experiment.adoptionFailure && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'adopt'}
            onClick={() =>
              run('adopt', () => actions.decideWrite(experiment.winnerCandidateId, true))
            }
          >
            {t('actions.retryAdopt')}
          </Button>
        )}
        {survivor && (
          <Button
            variant="cta"
            disabled={actions.isPending || voiceWorkBlocked}
            pending={pressed === 'useSingle'}
            onClick={() => run('useSingle', () => actions.useSingle(activeCandidateId))}
          >
            {t('actions.useSingle')}
          </Button>
        )}
        {canChoose && experiment.stage !== 'write' && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            pending={pressed === 'choose'}
            onClick={() => run('choose', () => actions.choose(activeCandidateId))}
          >
            {t('actions.choose')}
          </Button>
        )}
        {canChoose && experiment.stage === 'write' && (
          /* The write decision is the one status that offers TWO committing actions, and stacking
             both full-width put 100px of dock over the draft they are about. Side by side on the
             phone — the plain apply left, the one that also moves the active model right (§4) —
             halves that; `sm:contents` dissolves the pair back into the desktop row. 결과 적용하고
             활성 모델로 변경 wraps to two lines in a 146px column, which the tighter line box the
             Button primitive carries keeps inside the 44px floor. */
          <div className="grid grid-cols-2 gap-3 sm:contents">
            <Button
              variant="secondary"
              disabled={actions.isPending || voiceWorkBlocked}
              pending={pressed === 'decide'}
              onClick={() => run('decide', () => actions.decideWrite(activeCandidateId, false))}
            >
              {t('actions.apply')}
            </Button>
            <Button
              variant="cta"
              disabled={actions.isPending || voiceWorkBlocked}
              pending={pressed === 'decideAdopt'}
              onClick={() => run('decideAdopt', () => actions.decideWrite(activeCandidateId, true))}
            >
              {t('actions.applyAndAdopt')}
            </Button>
          </div>
        )}
        {experiment.status === 'decided' &&
          experiment.stage !== 'write' &&
          !experiment.appliedAt &&
          !experiment.applyFailure && (
            <Button
              variant="cta"
              disabled={actions.isPending || voiceWorkBlocked}
              pending={pressed === 'apply'}
              onClick={() =>
                experiment.stage === 'analyze'
                  ? setConfirmStyle(true)
                  : run('apply', () => actions.apply())
              }
            >
              {t('actions.apply')}
            </Button>
          )}
      </div>
      {/* The outcome renders inside the dock, right under the button that was pressed — a result
          reported 1,000px up the page has not been shown (§4.3). */}
      {actions.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={actions.failure} />
        </Notice>
      )}
      {experiment.voiceId && !voicesPending && voiceWorkBlocked && (
        <Notice tone="warning" role="status">
          {t('actions.voiceUnavailable')}
        </Notice>
      )}
      {experiment.applyFailure && (
        <Notice tone="danger" role="alert">
          <span>{t('actions.applyFailed')} </span>
          <AppFailureMessage failure={experiment.applyFailure} />
        </Notice>
      )}
      {experiment.appliedAt && !experiment.applyFailure && (
        <Notice tone="success" role="status">
          {t('actions.applied')}
          {experiment.stage === 'write' &&
            (experiment.adoptedAt ? t('actions.adopted') : t('actions.notAdopted'))}
        </Notice>
      )}
      {experiment.adoptionFailure && (
        <Notice tone="danger" role="alert">
          <span>{t('actions.adoptionFailed')} </span>
          <AppFailureMessage failure={experiment.adoptionFailure} />
        </Notice>
      )}
      <Dialog
        open={confirmStyle}
        title={t('actions.confirmStyleTitle')}
        confirmLabel={t('actions.confirmStyle')}
        pending={actions.isPending}
        onClose={() => setConfirmStyle(false)}
        onConfirm={() => {
          if (!voiceWorkBlocked) void actions.apply(true).then(() => setConfirmStyle(false))
        }}
      >
        {t('actions.confirmStyleDescription')}
      </Dialog>
    </div>
  )
}
