import { forwardRef, useCallback, useEffect, useImperativeHandle, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import {
  sameRef,
  useModelSetup,
  useSelectionSavePending,
  useStageSelection,
} from '@/entities/model-catalog'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, FieldMessage, Notice, buttonStyles } from '@/shared/ui'
import { useStartGeneration } from '../api/useStartGeneration'
import { useStartWriteExperiment } from '../api/useStartWriteExperiment'
import {
  comparisonGenerationPreconditions,
  ordinaryGenerationPreconditions,
  type GenerationModelSelection,
} from '../model/preconditions'
import { GenerationOptions } from './GenerationOptions'

export interface GenerationActionsHandle {
  startGeneration: () => void
  startComparison: () => void
}

export const GenerationActions = forwardRef<
  GenerationActionsHandle,
  {
    post: Pick<PostDraft, 'slug' | 'images' | 'pendingExperimentId' | 'targetLength' | 'voice'>
    activeJob?: GenerationJob
    jobPending?: boolean
    onStarted: (jobId: string) => void
    beforeStart?: () => Promise<void>
  }
>(function GenerationActions({ post, activeJob, jobPending = false, onStarted, beforeStart }, ref) {
  const { t } = useTranslation('posts')
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const setup = useModelSetup()
  const selectionSaving = useSelectionSavePending()
  const generation = useStartGeneration()
  const comparison = useStartWriteExperiment()
  const [preparing, setPreparing] = useState<'generation' | 'comparison' | ''>('')
  const [prepareFailure, setPrepareFailure] = useState<AppFailure>()
  const [targetLength, setTargetLength] = useState(post.targetLength)
  useEffect(() => setTargetLength(post.targetLength), [post.slug, post.targetLength])

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

  return (
    <div>
      {/* Two rows on a phone: the secondary pair shares one, and 생성 — the committing action —
          takes the whole width of the next, closest to the thumb (§4.3). `sm:contents` dissolves
          the pair's wrapper at the pointer breakpoint so all three sit in one row again. */}
      <div className="grid gap-3 sm:flex sm:flex-wrap sm:items-center">
        <div className="flex gap-3 sm:contents">
          <GenerationOptions
            key={`${post.slug}-${targetLength ?? 'natural'}`}
            slug={post.slug}
            targetLength={targetLength}
            disabled={busy}
            onSaved={setTargetLength}
          />
          <Button
            variant="secondary"
            className="flex-1 sm:flex-none"
            disabled={sharedDisabled || !ab.ok}
            pending={preparing === 'comparison' || comparison.isPending}
            onClick={() => void start('comparison')}
          >
            {t('generation.compare')}
          </Button>
        </div>
        <Button
          variant="cta"
          className="w-full sm:w-auto"
          disabled={sharedDisabled || !ordinary.ok}
          pending={preparing === 'generation' || generation.isPending}
          onClick={() => void start('generation')}
        >
          {t('generation.generate')}
        </Button>
      </div>
      <div className="mt-2 grid gap-1 text-sm">
        {(sharedReason || !ordinary.ok) && (
          <p role="status" className="text-content-secondary">
            {t('generation.generateReason', {
              reason: sharedReason || ordinary.reason,
              interpolation: { escapeValue: false },
            })}
          </p>
        )}
        {(sharedReason || !ab.ok) && (
          <p role="status" className="text-content-secondary">
            {t('generation.compareReason', {
              reason: sharedReason || ab.reason,
              // This value is another catalog sentence, never user or model data. Avoid
              // double-escaping its slash into visible `&#x2F;` while global interpolation
              // escaping remains enabled for untrusted values.
              interpolation: { escapeValue: false },
            })}
          </p>
        )}
      </div>
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
