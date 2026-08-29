import { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import {
  sameRef,
  useModelSetup,
  useModels,
  useSelectionSavePending,
  useStageSelection,
} from '@/entities/model-catalog'
import { Button, FieldMessage } from '@/shared/ui'
import { useStartGeneration } from '../api/useStartGeneration'
import { generationPreconditions, type GenerationModelSelection } from '../model/preconditions'

interface GenerateButtonProps {
  post: Pick<PostDraft, 'slug' | 'images' | 'pendingExperimentId'>
  activeJob?: GenerationJob
  /** A job id was accepted but its first durable snapshot has not arrived yet. */
  jobPending?: boolean
  onStarted: (jobId: string) => void
  beforeStart?: () => Promise<void>
}

export interface GenerateButtonHandle {
  start: () => void
}

/** Starts one durable observe→write job using the account's explicit stage choices. */
export const GenerateButton = forwardRef<GenerateButtonHandle, GenerateButtonProps>(
  function GenerateButton({ post, activeJob, jobPending = false, onStarted, beforeStart }, ref) {
    const observe = useStageSelection('observe')
    const { models } = useModels()
    const setup = useModelSetup()
    const startGeneration = useStartGeneration()
    const selectionSaving = useSelectionSavePending()
    const [preparing, setPreparing] = useState(false)
    const [prepareError, setPrepareError] = useState(false)
    const observeSelection = resolveSelection(observe.models, observe.selected)
    const writePair = setup.pairs.find((pair) => pair.stage === 'write')
    const writeSelectionA = resolveSelection(models, writePair?.candidateA?.ref ?? null)
    const writeSelectionB = resolveSelection(models, writePair?.candidateB?.ref ?? null)
    const preconditions = generationPreconditions(
      post.images,
      observeSelection,
      writeSelectionA,
      writeSelectionB,
      activeJob,
    )
    const hasPendingExperiment = Boolean(post.pendingExperimentId)
    const modelPending = observe.isPending || setup.isPending || selectionSaving
    const disabled =
      modelPending ||
      jobPending ||
      hasPendingExperiment ||
      preparing ||
      startGeneration.isPending ||
      !preconditions.ok

    const start = useCallback(async () => {
      if (
        modelPending ||
        jobPending ||
        hasPendingExperiment ||
        !preconditions.ok ||
        !writeSelectionA ||
        !writeSelectionB
      )
        return
      setPreparing(true)
      setPrepareError(false)
      try {
        await beforeStart?.()
      } catch {
        setPrepareError(true)
        setPreparing(false)
        return
      }
      try {
        const response = await startGeneration.start(
          post.slug,
          post.images.length > 0 ? observeSelection?.ref : undefined,
          writeSelectionA.ref,
          writeSelectionB.ref,
        )
        onStarted(response.jobId)
      } catch {
        // The mutation owns and renders the provider/API error.
      } finally {
        setPreparing(false)
      }
    }, [
      modelPending,
      jobPending,
      hasPendingExperiment,
      beforeStart,
      observeSelection,
      onStarted,
      post.images.length,
      post.slug,
      preconditions.ok,
      startGeneration,
      writeSelectionA,
      writeSelectionB,
    ])

    useImperativeHandle(ref, () => ({ start: () => void start() }), [start])

    const blocker = hasPendingExperiment
      ? '먼저 대기 중인 AI 결과를 확인해 주세요.'
      : jobPending
        ? '생성 작업을 확인하는 중이에요.'
        : selectionSaving
          ? '모델 선택을 저장하는 중이에요.'
          : modelPending
            ? '모델 선택을 확인하는 중이에요.'
            : preconditions.reason

    return (
      <div>
        <Button variant="cta" disabled={disabled} onClick={() => void start()}>
          {preparing
            ? '글을 저장하는 중…'
            : startGeneration.isPending
              ? '생성을 시작하는 중…'
              : '생성'}
        </Button>
        {blocker && (
          <p role="status" className="text-content-tertiary mt-2 text-xs">
            {blocker}{' '}
            {hasPendingExperiment ? (
              <a
                href={`/ai-models/experiments/${encodeURIComponent(post.pendingExperimentId)}`}
                className="text-link-fg underline"
              >
                AI 결과 확인
              </a>
            ) : (
              <a href="/ai-models" className="text-link-fg underline">
                AI 모델 설정
              </a>
            )}
          </p>
        )}
        {startGeneration.isError && (
          <FieldMessage className="mt-2">{startGeneration.errorMessage}</FieldMessage>
        )}
        {prepareError && !startGeneration.isError && (
          <FieldMessage className="mt-2">글을 저장하지 못했어요.</FieldMessage>
        )}
      </div>
    )
  },
)

function resolveSelection(
  models: ReturnType<typeof useStageSelection>['models'],
  selected: ReturnType<typeof useStageSelection>['selected'],
): GenerationModelSelection | undefined {
  if (!selected) return undefined
  const model = models.find((candidate) => sameRef(candidate.ref, selected))
  return model ? { ref: selected, vision: model.vision } : undefined
}
