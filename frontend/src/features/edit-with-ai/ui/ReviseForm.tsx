import { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { useSelectionSavePending, useStageSelection } from '@/entities/model-catalog'
import { REVISION_INSTRUCTION_MAX_CHARS } from '@/shared/config'
import { Button, Checkbox, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'
import { useStartRevision } from '../api/useStartRevision'

interface ReviseFormProps {
  postSlug: string
  activeJob?: GenerationJob
  jobPending?: boolean
  onStarted: (jobId: string) => void
  beforeStart?: () => Promise<void>
}

export interface ReviseFormHandle {
  start: () => void
}

/** Replaces the current canonical content through one durable `revise` job. */
export const ReviseForm = forwardRef<ReviseFormHandle, ReviseFormProps>(function ReviseForm(
  { postSlug, activeJob, jobPending = false, onStarted, beforeStart },
  ref,
) {
  const [instruction, setInstruction] = useState('')
  const [saveAsRule, setSaveAsRule] = useState(false)
  const [prepareError, setPrepareError] = useState(false)
  const write = useStageSelection('write')
  const selectionSaving = useSelectionSavePending()
  const startRevision = useStartRevision()
  const hasActiveJob = Boolean(activeJob && !isTerminal(activeJob))
  const trimmed = instruction.trim()
  const disabled =
    write.isPending ||
    selectionSaving ||
    jobPending ||
    hasActiveJob ||
    !write.selected ||
    trimmed === '' ||
    startRevision.isPending

  const start = useCallback(async () => {
    if (disabled || !write.selected) return
    setPrepareError(false)
    try {
      await beforeStart?.()
    } catch {
      setPrepareError(true)
      return
    }
    try {
      const response = await startRevision.start(postSlug, trimmed, saveAsRule, write.selected)
      onStarted(response.jobId)
    } catch {
      // The mutation owns and renders the transport/provider error.
    }
  }, [beforeStart, disabled, onStarted, postSlug, saveAsRule, startRevision, trimmed, write.selected])

  useImperativeHandle(ref, () => ({ start: () => void start() }), [start])

  const blocker = jobPending
    ? '수정 작업을 확인하는 중이에요.'
    : hasActiveJob
      ? '다른 작업이 진행 중이에요.'
      : selectionSaving || write.isPending
        ? '작성 모델을 확인하는 중이에요.'
        : !write.selected
          ? '작성 모델을 선택하세요.'
          : trimmed === ''
            ? '수정 요청을 입력하세요.'
            : ''

  return (
    <section aria-labelledby="revision-heading" className="mt-10 pb-12">
      <h2 id="revision-heading" className="text-lg font-semibold tracking-tight">
        AI로 수정
      </h2>
      <form
        className="mt-4 space-y-3"
        onSubmit={(event) => {
          event.preventDefault()
          void start()
        }}
      >
        <FieldLabel htmlFor="revision-instruction">수정 요청</FieldLabel>
        {/* A textarea, not a single-line field: at 360px one line shows ~20 of the 500 permitted
            Hangul, so an ordinary instruction scrolled its own beginning out of sight while it was
            being typed. `autoGrow` keeps it out of the page's scroll (§4.4); Return now inserts a
            line instead of submitting, which is why `enterKeyHint` is the plain one. */}
        <Textarea
          id="revision-instruction"
          value={instruction}
          rows={3}
          autoGrow
          maxLength={REVISION_INSTRUCTION_MAX_CHARS}
          autoComplete="off"
          autoCapitalize="sentences"
          enterKeyHint="enter"
          disabled={hasActiveJob || jobPending || startRevision.isPending}
          placeholder="어떻게 고칠까요? 예: 더 짧게 · 존댓말로 · 카페 얘기 늘려줘"
          onChange={(event) => setInstruction(event.target.value)}
        />
        {/* The cap used to stop the keystrokes with nothing on screen explaining why. */}
        <p className="text-content-tertiary text-xs">
          {instruction.length}/{REVISION_INSTRUCTION_MAX_CHARS}
        </p>
        <label className="text-content-secondary flex min-h-11 items-center gap-3 text-sm">
          <Checkbox
            checked={saveAsRule}
            disabled={hasActiveJob || jobPending || startRevision.isPending}
            onChange={(event) => setSaveAsRule(event.target.checked)}
          />
          이 요청을 규칙으로 저장
        </label>
        {/* Validation and failure sit ABOVE the action, so the keyboard covering the bottom ~40%
            of the screen hides at most the button and never the reason it is disabled (§8.3). */}
        {blocker && (
          <p role="status" className="text-content-secondary text-sm">
            {blocker}
          </p>
        )}
        {startRevision.isError && <FieldMessage>{startRevision.errorMessage}</FieldMessage>}
        {prepareError && <FieldMessage>편집한 글을 먼저 저장하지 못했어요.</FieldMessage>}
        <Button
          type="submit"
          variant="secondary"
          className="w-full sm:w-auto"
          disabled={disabled}
          pending={startRevision.isPending}
        >
          수정
        </Button>
      </form>
    </section>
  )
})
