import { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { useSelectionSavePending, useStageSelection } from '@/entities/model-catalog'
import { ContentRevisionConflictError } from '@/entities/post'
import { deletedVoiceAIReason, type VoiceRef } from '@/entities/voice'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { REVISION_INSTRUCTION_MAX_CHARS } from '@/shared/config'
import {
  AppFailureMessage,
  Button,
  Checkbox,
  FieldLabel,
  FieldMessage,
  Notice,
  Textarea,
} from '@/shared/ui'
import { useStartRevision } from '../api/useStartRevision'
import { SaveAsGuidelineButton } from './SaveAsGuidelineButton'

interface ReviseFormProps {
  ownerId: string
  postSlug: string
  /** The post's voice: a deleted one refuses revision before any provider call. */
  voice: Pick<VoiceRef, 'id' | 'deleted'>
  /** The revision itself stays available; only publishing its sentence as a voice rule is unsafe. */
  ruleLanguageMismatch?: boolean
  /** The post's current purpose, read from the already-loaded post so the guideline capture can
   *  offer it as a scope without issuing a query. Empty id means the post has none. */
  purpose?: { id: string; name: string }
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
  {
    ownerId,
    postSlug,
    voice,
    ruleLanguageMismatch = false,
    purpose,
    activeJob,
    jobPending = false,
    onStarted,
    beforeStart,
  },
  ref,
) {
  const { t } = useTranslation('posts')
  const [instruction, setInstruction] = useState('')
  const [saveAsRule, setSaveAsRule] = useState(false)
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  const write = useStageSelection('write')
  const selectionSaving = useSelectionSavePending()
  const startRevision = useStartRevision(ownerId, voice.id)
  const hasActiveJob = Boolean(activeJob && !isTerminal(activeJob))
  // A completed REVISION, not just any completed job: a finished `generate` job leaves the
  // instruction box holding text that never ran, and 'done' rather than merely terminal because a
  // failed revision produced nothing worth turning into a rule.
  const revisionCompleted = activeJob?.kind === 'revise' && activeJob.status === 'done'
  const voiceBlocked = Boolean(voice?.deleted)
  const trimmed = instruction.trim()
  const disabled =
    voiceBlocked ||
    write.isPending ||
    selectionSaving ||
    jobPending ||
    hasActiveJob ||
    !write.selected ||
    trimmed === '' ||
    startRevision.isPending

  const start = useCallback(async () => {
    if (disabled || !write.selected) return
    setPrepareFailure(undefined)
    try {
      await beforeStart?.()
    } catch (cause) {
      setPrepareFailure(
        cause instanceof ContentRevisionConflictError
          ? 'content-conflict'
          : appFailureFromConnect(cause),
      )
      return
    }
    try {
      const response = await startRevision.start(
        postSlug,
        trimmed,
        saveAsRule && !ruleLanguageMismatch,
        write.selected,
      )
      onStarted(response.jobId)
    } catch {
      // The mutation owns and renders the transport/provider error.
    }
  }, [
    beforeStart,
    disabled,
    onStarted,
    postSlug,
    ruleLanguageMismatch,
    saveAsRule,
    startRevision,
    trimmed,
    write.selected,
  ])

  useImperativeHandle(ref, () => ({ start: () => void start() }), [start])

  const blocker = voiceBlocked
    ? deletedVoiceAIReason()
    : jobPending
      ? t('revision.blocked.jobChecking')
      : hasActiveJob
        ? t('revision.blocked.activeJob')
        : selectionSaving || write.isPending
          ? t('revision.blocked.modelChecking')
          : !write.selected
            ? t('revision.blocked.model')
            : trimmed === ''
              ? t('revision.blocked.instruction')
              : ''

  return (
    <section aria-labelledby="revision-heading" className="mt-10 pb-12">
      <h2 id="revision-heading" className="text-lg font-semibold tracking-tight">
        {t('revision.title')}
      </h2>
      <form
        className="mt-4 space-y-3"
        onSubmit={(event) => {
          event.preventDefault()
          void start()
        }}
      >
        <FieldLabel htmlFor="revision-instruction">{t('revision.instruction')}</FieldLabel>
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
          disabled={voiceBlocked || hasActiveJob || jobPending || startRevision.isPending}
          placeholder={t('revision.placeholder')}
          onChange={(event) => setInstruction(event.target.value)}
        />
        {/* The cap used to stop the keystrokes with nothing on screen explaining why. */}
        <p className="text-content-tertiary text-xs">
          {instruction.length}/{REVISION_INSTRUCTION_MAX_CHARS}
        </p>
        <label className="text-content-secondary flex min-h-11 items-center gap-3 text-sm">
          <Checkbox
            checked={saveAsRule}
            disabled={
              voiceBlocked ||
              ruleLanguageMismatch ||
              hasActiveJob ||
              jobPending ||
              startRevision.isPending
            }
            onChange={(event) => setSaveAsRule(event.target.checked)}
          />
          {t('revision.saveAsRule')}
        </label>
        {ruleLanguageMismatch && (
          <p role="status" className="text-content-secondary text-sm">
            {t('revision.ruleLanguageMismatch')}
          </p>
        )}
        {/* Beside 규칙으로 저장, but only after a revision has actually finished: the instruction is
            worth saving as a rule once the user has seen what it did. `규칙으로 저장` has to be a
            pre-flight checkbox because the voice learns from the run itself; a guideline is a plain
            create, so it can wait for the result. */}
        {revisionCompleted && (
          <div className="flex flex-wrap items-center gap-2">
            <SaveAsGuidelineButton
              ownerId={ownerId}
              instruction={trimmed}
              purpose={purpose?.id ? purpose : undefined}
              disabled={trimmed === '' || startRevision.isPending}
            />
          </div>
        )}
        {/* Validation and failure sit ABOVE the action, so the keyboard covering the bottom ~40%
            of the screen hides at most the button and never the reason it is disabled (§8.3). */}
        {blocker && (
          <p role="status" className="text-content-secondary text-sm">
            {blocker}
          </p>
        )}
        {startRevision.isError && <FieldMessage>{startRevision.errorMessage}</FieldMessage>}
        {prepareFailure && (
          <Notice tone="danger" role="alert">
            {prepareFailure === 'content-conflict' ? (
              t('edit.conflict')
            ) : (
              <AppFailureMessage failure={prepareFailure} />
            )}
          </Notice>
        )}
        <Button
          type="submit"
          variant="secondary"
          className="w-full sm:w-auto"
          disabled={disabled}
          pending={startRevision.isPending}
        >
          {t('revision.submit')}
        </Button>
      </form>
    </section>
  )
})
