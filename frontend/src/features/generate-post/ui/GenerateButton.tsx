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
import { POST_TARGET_LENGTH_DEFAULT, POST_TARGET_LENGTH_MAX, POST_TARGET_LENGTH_MIN } from '@/shared/config'
import { Button, FieldLabel, FieldMessage, TextField, buttonStyles } from '@/shared/ui'
import { useStartGeneration } from '../api/useStartGeneration'
import { generationPreconditions, type GenerationModelSelection } from '../model/preconditions'

interface GenerateButtonProps {
  post: Pick<PostDraft, 'slug' | 'images' | 'pendingExperimentId' | 'targetLength'>
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
    const [targetLength, setTargetLength] = useState(post.targetLength || POST_TARGET_LENGTH_DEFAULT)
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
    const targetValid = targetLength >= POST_TARGET_LENGTH_MIN && targetLength <= POST_TARGET_LENGTH_MAX
    const disabled =
      modelPending ||
      jobPending ||
      hasPendingExperiment ||
      preparing ||
      startGeneration.isPending ||
      !targetValid ||
      !preconditions.ok

    const start = useCallback(async () => {
      if (
        modelPending ||
        jobPending ||
        hasPendingExperiment ||
        !preconditions.ok ||
        !targetValid ||
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
          targetLength,
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
      targetLength,
      targetValid,
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

    // The one escape from a blocked state. It stays a native anchor because this feature is also
    // rendered outside a RouterProvider, where a router `Link` throws; the editor's model grid
    // reaches the same destination with a real `Link`.
    const escapeLink = hasPendingExperiment
      ? {
          href: `/ai-models/experiments/${encodeURIComponent(post.pendingExperimentId)}`,
          label: 'AI 결과 확인',
        }
      : { href: '/ai-models', label: 'AI 모델 설정' }

    return (
      <div>
        <div className="mb-4">
          <FieldLabel htmlFor={`target-length-${post.slug}`}>목표 글자 수</FieldLabel>
          <TextField
            id={`target-length-${post.slug}`}
            type="number"
            min={POST_TARGET_LENGTH_MIN}
            max={POST_TARGET_LENGTH_MAX}
            value={targetLength}
            onChange={(event) => setTargetLength(Number(event.target.value))}
            aria-invalid={!targetValid || undefined}
            className="mt-1 max-w-40"
          />
          {!targetValid && (
            <FieldMessage className="mt-1">
              {POST_TARGET_LENGTH_MIN.toLocaleString()}–{POST_TARGET_LENGTH_MAX.toLocaleString()}자로 입력해 주세요.
            </FieldMessage>
          )}
        </div>
        {/* Full-bleed on the phone (§4, §7): '생성' is two Hangul, so a text-sized target is 52px
            wide for the action the whole screen exists for. `pending` also holds that width — the
            old label swap to '생성을 시작하는 중…' tripled the box under the thumb mid-press. */}
        <Button
          variant="cta"
          className="w-full sm:w-auto"
          disabled={disabled}
          pending={preparing || startGeneration.isPending}
          onClick={() => void start()}
        >
          생성
        </Button>
        {blocker && (
          <div className="mt-3">
            {/* Copy the user has to act on is never 12px (§3). */}
            <p role="status" className="text-content-secondary text-sm">
              {blocker}
            </p>
            {/* On its own line and wearing the button contract: inline in a 12px paragraph this
                was a 67×16 target. `secondary`, not `ghost`, because a transparent resting plane
                reads as loose words on a device with no hover (§6). */}
            <a
              href={escapeLink.href}
              className={buttonStyles({
                variant: 'secondary',
                className: 'mt-2 w-full sm:w-auto',
              })}
            >
              {escapeLink.label}
            </a>
          </div>
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
