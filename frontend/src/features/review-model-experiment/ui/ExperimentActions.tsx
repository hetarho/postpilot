import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ModelExperiment } from '@/entities/model-experiment'
import {
  candidateSides,
  needsExperimentReview,
  useExperimentActions,
  useExperimentOwnerRefresh,
} from '@/entities/model-experiment'
import { isUnfinalized, usePost } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import type { CandidateBadges } from '@/entities/model-experiment'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'
import { hasExperimentActions } from '../model/experiment-actions'
import { VerdictSheet } from './VerdictSheet'
import { RankedReviewSheet } from './RankedReviewSheet'

export function ExperimentActions({
  experiment,
  activeCandidateId,
}: {
  experiment: ModelExperiment
  activeCandidateId: string
}) {
  const { t } = useTranslation('models')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { voices, isPending: voicesPending } = useVoices(ownerId)
  const frozenVoice = experiment.voiceId
    ? voices.find((voice) => voice.id === experiment.voiceId)
    : undefined
  // A write comparison's retry writes again in its frozen voice and its application writes the
  // post in it. Keep account-scoped decisions and model adoption available, but never let
  // provider or post work target a tombstone (or an unverified route cache entry).
  const voiceWorkBlocked = Boolean(
    experiment.voiceId && (voicesPending || !frozenVoice || frozenVoice.deleted),
  )
  const refreshOwner = useExperimentOwnerRefresh(experiment)
  const actions = useExperimentActions(experiment.id, refreshOwner)
  // Which winner action is waiting on its sheet. Non-empty IS the sheet's open state: there
  // is one sheet, and what it confirms is whichever action opened it (MODEL-61).
  const [verdict, setVerdict] = useState<'choose' | 'decide' | 'decideAdopt' | ''>('')
  // `useExperimentActions` reports one `isPending` for all six mutations, so the button the thumb
  // actually pressed has to be remembered here — otherwise the whole bar spins at once. Without a
  // pending state at all, a tap on a cellular connection only dropped the button to 50% opacity,
  // which is not feedback on a device with no hover, and the user fired the mutation twice (THEME-28).
  const [pressed, setPressed] = useState('')
  const run = (name: string, action: () => Promise<unknown>) => {
    setPressed(name)
    void action()
      .catch(() => {})
      .finally(() => setPressed(''))
  }
  // A 말투 반영 비교 wrote nothing anywhere: its only follow-up is adopting the winner (MODEL-36).
  const writesNothing = experiment.source === 'voice'
  const selected = experiment.candidates.find((candidate) => candidate.id === activeCandidateId)
  const survivor =
    !writesNothing && experiment.status === 'partial' && selected?.status === 'succeeded'
  const canChoose = experiment.status === 'review' && selected?.status === 'succeeded'
  // The editor's write comparison wrote a post that holds no content until one side is
  // applied, so its verdict commits. Every lab comparison picks a winner and applies
  // nothing; what it may still write is offered afterwards, one action at a time.
  const commits = experiment.stage === 'write' && experiment.origin === 'editor'
  const offersContent =
    !writesNothing &&
    experiment.origin === 'lab' &&
    (experiment.stage === 'write' || experiment.stage === 'observe')
  // Asked for only where the answer changes the screen: what a decided lab comparison may
  // still write to. A finalized post keeps its confirmed content, so it is offered nothing.
  const { post, isPending: postPending } = usePost(experiment.postSlug, {
    enabled:
      !writesNothing &&
      !experiment.appliedAt &&
      ((offersContent && experiment.status === 'decided') ||
        (experiment.reviewMode === 'candidate_ranking' && experiment.status === 'completed')),
  })
  const postWritable = Boolean(post && isUnfinalized(post))
  if (experiment.reviewMode === 'candidate_ranking') {
    return (
      <RankedExperimentActions
        experiment={experiment}
        actions={actions}
        voiceWorkBlocked={voiceWorkBlocked}
        activeCandidateId={activeCandidateId}
        postWritable={postWritable}
        postPending={postPending}
      />
    )
  }
  if (!hasExperimentActions(experiment)) return null
  return (
    <div className="grid gap-3">
      {/* Full-width targets on a phone: three Korean labels measure ~410px against the 296px the
          bar has at 360px, so a wrapping row became two right-aligned rows of ambiguous targets
          8px apart (THEME-23). One row per action is the default; the write decision's two committing
          actions pair off into a single row of their own below, because three stacked rows of
          chrome hide the draft the decision is about. From `sm:` up everything collapses back
          into the desktop row. The CTA is the last child in every status (THEME-22). */}
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
                commits
                  ? actions.decideWrite(experiment.winnerCandidateId, experiment.adoptionRequested)
                  : actions.apply(),
              )
            }
          >
            {t('actions.retryApply')}
          </Button>
        )}
        {/* The adoption leaves its marker, so a reload never offers it again (MODEL-36); a
            failed one keeps this button as its retry. */}
        {experiment.status === 'decided' && !commits && !experiment.adoptedAt && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'adopt'}
            onClick={() => run('adopt', actions.adopt)}
          >
            {t('actions.useActive')}
          </Button>
        )}
        {experiment.adoptionFailure && commits && (
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
        {canChoose && !commits && (
          <Button
            variant="cta"
            disabled={actions.isPending}
            pending={pressed === 'choose'}
            onClick={() => setVerdict('choose')}
          >
            {t('actions.choose')}
          </Button>
        )}
        {canChoose && commits && (
          /* The write decision is the one status that offers TWO committing actions, and stacking
             both full-width put 100px of dock over the draft they are about. Side by side on the
             phone — the plain apply left, the one that also moves the active model right (THEME-22) —
             halves that; `sm:contents` dissolves the pair back into the desktop row. 결과 적용하고
             활성 모델로 변경 wraps to two lines in a 146px column, which the tighter line box the
             Button primitive carries keeps inside the 44px floor. */
          <div className="grid grid-cols-2 gap-3 sm:contents">
            <Button
              variant="secondary"
              disabled={actions.isPending || voiceWorkBlocked}
              pending={pressed === 'decide'}
              onClick={() => setVerdict('decide')}
            >
              {t('actions.apply')}
            </Button>
            <Button
              variant="cta"
              disabled={actions.isPending || voiceWorkBlocked}
              pending={pressed === 'decideAdopt'}
              onClick={() => setVerdict('decideAdopt')}
            >
              {t('actions.applyAndAdopt')}
            </Button>
          </div>
        )}
        {experiment.status === 'decided' &&
          !commits &&
          !writesNothing &&
          !experiment.appliedAt &&
          !experiment.applyFailure &&
          // A comparison that writes to a post offers it only while that post still takes
          // writing, and never while the answer is still on its way.
          (offersContent ? postWritable && !postPending : true) && (
            <Button
              variant="cta"
              disabled={actions.isPending || voiceWorkBlocked}
              pending={pressed === 'apply'}
              onClick={() => run('apply', () => actions.apply())}
            >
              {t('actions.apply')}
            </Button>
          )}
      </div>
      {/* The outcome renders inside the dock, right under the button that was pressed — a result
          reported 1,000px up the page has not been shown (THEME-24). */}
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
      {/* One sheet for all three winner actions. Dismissal and the single-survivor
          application open none: nothing was chosen, so there is nothing to explain. */}
      <VerdictSheet
        experiment={experiment}
        chosenCandidateId={activeCandidateId}
        open={Boolean(verdict)}
        pending={Boolean(pressed)}
        title={t(verdict === 'choose' ? 'actions.choose' : 'actions.apply')}
        confirmLabel={t(
          verdict === 'decideAdopt'
            ? 'actions.applyAndAdopt'
            : verdict === 'decide'
              ? 'actions.apply'
              : 'actions.choose',
        )}
        onClose={() => setVerdict('')}
        onConfirm={(badges: CandidateBadges[]) => {
          const committing = verdict
          setVerdict('')
          if (committing === 'choose') {
            run('choose', () => actions.choose(activeCandidateId, badges))
            return
          }
          run(committing, () =>
            actions.decideWrite(activeCandidateId, committing === 'decideAdopt', badges),
          )
        }}
      />
    </div>
  )
}

function RankedExperimentActions({
  experiment,
  actions,
  voiceWorkBlocked,
  activeCandidateId,
  postWritable,
  postPending,
}: {
  experiment: ModelExperiment
  actions: ReturnType<typeof useExperimentActions>
  voiceWorkBlocked: boolean
  activeCandidateId: string
  postWritable: boolean
  postPending: boolean
}) {
  const { t } = useTranslation('models')
  const [open, setOpen] = useState(false)
  const [pressed, setPressed] = useState('')
  const pendingReview = needsExperimentReview(experiment.status)
  const succeeded = experiment.candidates.filter((candidate) => candidate.status === 'succeeded')
  const selected = succeeded.find((candidate) => candidate.id === activeCandidateId)
  const sides = candidateSides(experiment.candidates)
  const labelOf = (id: string) => sides.find(({ candidate }) => candidate.id === id)?.label ?? '?'
  const appliedId = experiment.appliedCandidateId ?? ''
  const adoptedId = experiment.adoptedCandidateId ?? ''
  const applyTarget = appliedId || selected?.id || ''
  const adoptTarget = adoptedId || selected?.id || ''
  const completed = experiment.status === 'completed'
  const canApply = completed && experiment.source !== 'voice' && !experiment.appliedAt
  const canAdopt =
    completed &&
    !experiment.adoptedAt &&
    (experiment.origin === 'lab' || Boolean(experiment.appliedAt))
  const readOnly = canApply && !postPending && !postWritable
  const run = (kind: string, action: () => Promise<unknown>) => {
    setPressed(kind)
    void action()
      .catch(() => {})
      .finally(() => setPressed(''))
  }
  if (!pendingReview && !completed) return null
  return (
    <div className="grid gap-3">
      {completed && selected && (
        <Notice tone="info" role="status">
          {t('ranking.actionCandidate', { label: labelOf(selected.id) })}
        </Notice>
      )}
      {completed && !selected && (
        <Notice tone="warning" role="status">
          {t('ranking.selectedFailed')}
        </Notice>
      )}
      <div className="grid gap-3 sm:flex sm:flex-wrap sm:justify-end">
        {pendingReview && (experiment.status === 'partial' || experiment.status === 'failed') && (
          <Button
            variant="secondary"
            disabled={actions.isPending || voiceWorkBlocked}
            pending={pressed === 'retry'}
            onClick={() => run('retry', actions.retry)}
          >
            {t('actions.retryFailed')}
          </Button>
        )}
        {pendingReview && succeeded.length > 0 && (
          <Button
            variant="ghost"
            disabled={actions.isPending}
            pending={pressed === 'skip'}
            onClick={() => run('skip', () => actions.complete([], true))}
          >
            {t('ranking.skip')}
          </Button>
        )}
        {pendingReview && succeeded.length >= 2 && (
          <Button variant="cta" disabled={actions.isPending} onClick={() => setOpen(true)}>
            {t('ranking.open')}
          </Button>
        )}
        {canApply &&
          applyTarget &&
          !postPending &&
          postWritable &&
          !voiceWorkBlocked &&
          (experiment.origin === 'editor' && !appliedId ? (
            <div className="grid grid-cols-2 gap-3 sm:contents" key="editor-apply">
              <Button
                variant="secondary"
                disabled={actions.isPending}
                pending={pressed === 'apply'}
                onClick={() => run('apply', () => actions.applyCandidate(applyTarget))}
              >
                {t('actions.apply')}
              </Button>
              <Button
                variant="cta"
                disabled={actions.isPending}
                pending={pressed === 'applyAdopt'}
                onClick={() => run('applyAdopt', () => actions.applyCandidate(applyTarget, true))}
              >
                {t('actions.applyAndAdopt')}
              </Button>
            </div>
          ) : (
            <Button
              variant="cta"
              disabled={actions.isPending}
              pending={pressed === 'apply'}
              onClick={() =>
                run('apply', () =>
                  actions.applyCandidate(applyTarget, experiment.adoptionRequested),
                )
              }
            >
              {appliedId
                ? t('ranking.retryApplyCandidate', { label: labelOf(appliedId) })
                : t('actions.apply')}
            </Button>
          ))}
        {canAdopt && adoptTarget && !voiceWorkBlocked && (
          <Button
            variant="secondary"
            disabled={actions.isPending}
            pending={pressed === 'adopt'}
            onClick={() =>
              run('adopt', () =>
                experiment.adoptionRequested && appliedId
                  ? actions.applyCandidate(appliedId, true)
                  : actions.adoptCandidate(adoptTarget),
              )
            }
          >
            {adoptedId || experiment.adoptionRequested
              ? t('ranking.retryAdoptCandidate', { label: labelOf(adoptedId || appliedId) })
              : t('actions.useActive')}
          </Button>
        )}
      </div>
      {readOnly && (
        <Notice tone="warning" role="status">
          {t('ranking.postReadOnly')}
        </Notice>
      )}
      {completed && voiceWorkBlocked && (
        <Notice tone="warning" role="status">
          {t('actions.voiceUnavailable')}
        </Notice>
      )}
      {experiment.applyFailure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={experiment.applyFailure} />
        </Notice>
      )}
      {experiment.adoptionFailure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={experiment.adoptionFailure} />
        </Notice>
      )}
      {completed && experiment.appliedAt && appliedId && (
        <Notice tone="success" role="status">
          {t('ranking.appliedCandidate', {
            label: labelOf(appliedId),
            when: new Date(experiment.appliedAt).toLocaleString(),
          })}
        </Notice>
      )}
      {completed && experiment.adoptedAt && adoptedId && (
        <Notice tone="success" role="status">
          {t('ranking.adoptedCandidate', {
            label: labelOf(adoptedId),
            when: new Date(experiment.adoptedAt).toLocaleString(),
          })}
        </Notice>
      )}
      {actions.failure && !open && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={actions.failure} />
        </Notice>
      )}
      {pendingReview && (
        <RankedReviewSheet
          experiment={experiment}
          open={open}
          pending={actions.isPending}
          failure={actions.failure}
          onConfirm={(ranks) => actions.complete(ranks)}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  )
}
