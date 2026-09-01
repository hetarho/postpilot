import { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Link } from '@tanstack/react-router'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import {
  sameRef,
  useModelSetup,
  useSelectionSavePending,
  useStageSelection,
} from '@/entities/model-catalog'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  FieldMessage,
  Notice,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { useStartGeneration } from '../api/useStartGeneration'
import { useStartWriteExperiment } from '../api/useStartWriteExperiment'
import {
  comparisonGenerationPreconditions,
  isSetupBlocker,
  ordinaryGenerationPreconditions,
  setupBlockerTarget,
  type GenerationModelSelection,
} from '../model/preconditions'

export interface GenerationActionsHandle {
  startGeneration: () => void
  startComparison: () => void
}

export const GenerationActions = forwardRef<
  GenerationActionsHandle,
  {
    post: Pick<PostDraft, 'slug' | 'images' | 'pendingExperimentId' | 'voice'>
    /** Owned by the editor, not by this action: the writing brief sets it from another layer
     *  (`widgets/generation-brief`) and the two must agree on what the next run is given. */
    targetLength?: number
    activeJob?: GenerationJob
    jobPending?: boolean
    onStarted: (jobId: string) => void
    beforeStart?: () => Promise<void>
    /** Opens the writing brief, which is where the active 관찰/작성 모델 are chosen. Supplied by
     *  `pages/editor`, which owns the brief's open state — a feature may not import the widget
     *  that composes its siblings (ARCHITECTURE §3). */
    onOpenBrief?: () => void
  }
>(function GenerationActions(
  { post, targetLength, activeJob, jobPending = false, onStarted, beforeStart, onOpenBrief },
  ref,
) {
  const { t } = useTranslation('posts')
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const setup = useModelSetup()
  const selectionSaving = useSelectionSavePending()
  const generation = useStartGeneration()
  const comparison = useStartWriteExperiment()
  const [preparing, setPreparing] = useState<'generation' | 'comparison' | ''>('')
  const [prepareFailure, setPrepareFailure] = useState<AppFailure>()

  const observeSelection = resolveSelection(observe.models, observe.selected)
  const writeSelection = resolveSelection(write.models, write.selected)
  const pair = setup.pairs.find((value) => value.stage === 'write')
  const writeA = resolveSelection(write.models, pair?.candidateA?.ref ?? null)
  const writeB = resolveSelection(write.models, pair?.candidateB?.ref ?? null)
  const ordinary = ordinaryGenerationPreconditions(
    post.images,
    observeSelection,
    writeSelection,
    activeJob,
    post.voice,
  )
  const ab = comparisonGenerationPreconditions(
    post.images,
    observeSelection,
    writeA,
    writeB,
    activeJob,
    post.voice,
  )
  const pendingExperiment = Boolean(post.pendingExperimentId)
  const modelPending = observe.isPending || write.isPending || setup.isPending || selectionSaving
  const busy = jobPending || Boolean(preparing) || generation.isPending || comparison.isPending
  const sharedDisabled = modelPending || busy || pendingExperiment

  const start = useCallback(
    async (mode: 'generation' | 'comparison') => {
      const precondition = mode === 'generation' ? ordinary : ab
      if (sharedDisabled || !precondition.ok) return
      if (mode === 'generation' && !writeSelection) return
      if (mode === 'comparison' && (!writeA || !writeB)) return
      setPreparing(mode)
      setPrepareFailure(undefined)
      try {
        await beforeStart?.()
      } catch (cause) {
        setPrepareFailure(appFailureFromConnect(cause))
        setPreparing('')
        return
      }
      try {
        const response =
          mode === 'generation'
            ? await generation.start(
                post.slug,
                post.images.length ? observeSelection?.ref : undefined,
                writeSelection!.ref,
                targetLength,
              )
            : await comparison.start(
                post.slug,
                post.images.length ? observeSelection?.ref : undefined,
                writeA!.ref,
                writeB!.ref,
                targetLength,
              )
        onStarted(response.jobId)
      } catch {
        // The mode-specific mutation renders its transport error below the actions.
      } finally {
        setPreparing('')
      }
    },
    [
      ab,
      beforeStart,
      comparison,
      generation,
      observeSelection,
      onStarted,
      ordinary,
      post.images.length,
      post.slug,
      sharedDisabled,
      targetLength,
      writeA,
      writeB,
      writeSelection,
    ],
  )

  useImperativeHandle(
    ref,
    () => ({
      startGeneration: () => void start('generation'),
      startComparison: () => void start('comparison'),
    }),
    [start],
  )

  const sharedReason = pendingExperiment
    ? t('generation.pendingExperiment')
    : jobPending
      ? t('generation.jobChecking')
      : selectionSaving
        ? t('generation.selectionSaving')
        : modelPending
          ? t('generation.selectionChecking')
          : ''

  // Nothing this step can start yet, and the only reason is that the models have never been
  // chosen. Two dead buttons are not an empty state (§7): the bar drops them and offers the way to
  // the surface each missing piece is actually chosen on — the brief for the active selections,
  // the AI 모델 page for the A/B pair. Any other blocker (a job in flight, a deleted voice, a
  // selection still loading) keeps the ordinary disabled buttons, because waiting IS the answer.
  const blockedOnSetup =
    !modelPending &&
    !busy &&
    !pendingExperiment &&
    isSetupBlocker(ordinary.blocker) &&
    isSetupBlocker(ab.blocker)
  const needsBrief =
    blockedOnSetup &&
    Boolean(onOpenBrief) &&
    (setupBlockerTarget(ordinary.blocker) === 'brief' || setupBlockerTarget(ab.blocker) === 'brief')
  const needsPair =
    blockedOnSetup &&
    (setupBlockerTarget(ordinary.blocker) === 'models' ||
      setupBlockerTarget(ab.blocker) === 'models')

  // One line per action, in the action's own words. Rendered ABOVE the controls in the setup
  // state, where it is the message and the buttons are the answer to it, and below them otherwise,
  // where it explains a control the user can already see.
  const reasons = (className?: string) => (
    <div className={clsx('grid gap-1 empty:hidden', className)}>
      {(sharedReason || !ordinary.ok) && (
        <Typography variant="label" as="p" role="status">
          {t('generation.generateReason', {
            reason: sharedReason || ordinary.reason,
            interpolation: { escapeValue: false },
          })}
        </Typography>
      )}
      {(sharedReason || !ab.ok) && (
        <Typography variant="label" as="p" role="status">
          {t('generation.compareReason', {
            reason: sharedReason || ab.reason,
            // This value is another catalog sentence, never user or model data. Avoid
            // double-escaping its slash into visible `&#x2F;` while global interpolation
            // escaping remains enabled for untrusted values.
            interpolation: { escapeValue: false },
          })}
        </Typography>
      )}
    </div>
  )

  if (blockedOnSetup) {
    return (
      <div className="grid gap-3">
        {reasons()}
        <div className="grid gap-3 sm:flex sm:flex-wrap sm:justify-end">
          {needsBrief && (
            <Button variant="secondary" onClick={onOpenBrief}>
              {t('generation.setup.brief')}
            </Button>
          )}
          {needsPair && (
            <Link to="/ai-models" className={buttonStyles({ variant: 'secondary' })}>
              {t('generation.setup.pair')}
            </Link>
          )}
        </div>
      </div>
    )
  }

  return (
    <div>
      {/* ONE row on a phone, halves: A/B 비교 left, 생성 — the committing action — right, which is
          both the §4 emphasis order and the side the thumb of a right-handed one-handed grip
          reaches first. The writing brief no longer shares this row; it is the dock's top-right
          glyph, so the two things the step actually starts are the only full-size targets here.
          From `sm:` up the pair right-aligns at its natural width. */}
      <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap sm:items-center sm:justify-end">
        <Button
          variant="secondary"
          disabled={sharedDisabled || !ab.ok}
          pending={preparing === 'comparison' || comparison.isPending}
          onClick={() => void start('comparison')}
        >
          {t('generation.compare')}
        </Button>
        <Button
          variant="cta"
          disabled={sharedDisabled || !ordinary.ok}
          pending={preparing === 'generation' || generation.isPending}
          onClick={() => void start('generation')}
        >
          {t('generation.generate')}
        </Button>
      </div>
      {reasons('mt-2')}
      {pendingExperiment && (
        <a
          href={`/ai-models/experiments/${encodeURIComponent(post.pendingExperimentId)}`}
          className={buttonStyles({ variant: 'secondary', className: 'mt-2 w-full sm:w-auto' })}
        >
          {t('generation.reviewResult')}
        </a>
      )}
      {(generation.isError || comparison.isError) && (
        <FieldMessage className="mt-2">
          {generation.errorMessage || comparison.errorMessage}
        </FieldMessage>
      )}
      {prepareFailure && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={prepareFailure} />
        </Notice>
      )}
    </div>
  )
})

function resolveSelection(
  models: ReturnType<typeof useStageSelection>['models'],
  selected: ReturnType<typeof useStageSelection>['selected'],
): GenerationModelSelection | undefined {
  if (!selected) return undefined
  const model = models.find((candidate) => sameRef(candidate.ref, selected))
  return model ? { ref: selected, vision: model.vision } : undefined
}
