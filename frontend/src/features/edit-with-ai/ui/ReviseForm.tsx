import { forwardRef, useCallback, useImperativeHandle, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { SendHorizontal } from 'lucide-react'
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
  FieldMessage,
  Notice,
  Textarea,
  Typography,
  typographyStyles,
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
  /** Rendered at the top-right of the row's own heading. `widgets/refine-dock` puts 확정하기 there:
   *  the step's way OUT belongs beside the name of the loop it leaves, not underneath the field
   *  that continues it. A slot rather than an import, because a feature may not reach a sibling
   *  feature (ARCHITECTURE §3). */
  action?: ReactNode
}

export interface ReviseFormHandle {
  start: () => void
}

/** Replaces the current canonical content through one durable `revise` job.
 *
 *  It is the body of 글 다듬기's dock (`widgets/refine-dock`) and renders no surface of its own: a
 *  4,000px draft used to put this form past the end of the page, which is exactly where §4.3 says
 *  a committing action may not live. It DOES render the row's heading, with the step's way out
 *  (확정하기) in the `action` slot beside it.
 *
 *  Its SECONDARY controls — the counter, 규칙으로 저장 and 지침으로 저장 — collapse while the field
 *  is empty and unfocused. The dock is over the draft the whole time, so the row that is not being
 *  used is height taken from the thing the screen is for (§0). They come back on focus, on the
 *  first character, and for as long as a revision is running or has failed, because that is when
 *  their state is worth reading. */
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
    action,
  },
  ref,
) {
  const { t } = useTranslation('posts')
  const [instruction, setInstruction] = useState('')
  const [saveAsRule, setSaveAsRule] = useState(false)
  const [focused, setFocused] = useState(false)
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  const write = useStageSelection('write')
  const selectionSaving = useSelectionSavePending()
  const startRevision = useStartRevision(ownerId, voice.id)
  const hasActiveJob = Boolean(activeJob && !isTerminal(activeJob))
  // A completed REVISION, not just any completed job: a finished `generate` job leaves the
  // instruction box holding text that never ran, and 'done' rather than merely terminal because a
  // failed revision produced nothing worth turning into a rule.
  const revisionCompleted = activeJob?.kind === 'revise' && activeJob.status === 'done'
  const revisionBusy =
    activeJob?.kind === 'revise' && (!isTerminal(activeJob) || activeJob.status === 'failed')
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
  const expanded = focused || instruction !== '' || revisionBusy

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
            : ''

  return (
    <form
      className="grid gap-2"
      onSubmit={(event) => {
        event.preventDefault()
        void start()
      }}
    >
      {/* The row's own heading, with the step's way out beside it. The label is VISIBLE: it used
          to be `sr-only`, which left the field's prompt to a placeholder — `meta`-sized,
          `content-tertiary`, and gone the moment a character was typed — so the one bar holding
          both the revision loop and the actions that end it named neither of them (owner decision
          2026-09-02). It is still the field's own `<label>`, so the accessible name is the text
          the user can read.
          `fieldTitle`, not `title`: this is a field's name standing beside the step's way out, not
          a second step heading, so it is smaller than the step title and heavier than a caption
          (§3). The action slot takes the ROW'S whole remaining width — 확정하기 is two words, and
          at its natural size it read as an afterthought next to the label instead of as the
          thing that ends the step. */}
      <div className="flex items-center justify-between gap-3">
        <Typography
          variant="fieldTitle"
          as="label"
          htmlFor="revision-instruction"
          className="min-w-0"
        >
          {t('revision.instruction')}
        </Typography>
        {action}
      </div>
      {/* Validation and failure sit ABOVE the controls, so the keyboard covering the bottom ~40%
          of the screen hides at most a button and never the reason it is disabled (§8.3). */}
      {blocker && (
        <Typography variant="body" role="status" className="text-content-secondary">
          {blocker}
        </Typography>
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
      {/* The focus tracking is on the CONTROLS, not on the form: the heading row holds 확정하기,
          and pressing it must not expand the secondary row underneath the field it never touched. */}
      <div
        className="grid gap-2"
        onFocus={() => setFocused(true)}
        onBlur={(event) => {
          // Only a focus move OUT of this group collapses the row: tabbing from the field onto the
          // 규칙으로 저장 checkbox inside it must not unmount the checkbox mid-gesture.
          if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false)
        }}
      >
        <div className="flex items-end gap-2">
          {/* A textarea, not a single-line field: at 360px one line shows ~20 of the 500 permitted
            Hangul, so an ordinary instruction scrolled its own beginning out of sight while it was
            being typed. `autoGrow` keeps it out of the page's scroll (§4.4); Return inserts a line
            instead of submitting, which is why `enterKeyHint` is the plain one — the send button
            beside it is how the instruction is committed. */}
          <Textarea
            id="revision-instruction"
            value={instruction}
            rows={1}
            autoGrow
            maxLength={REVISION_INSTRUCTION_MAX_CHARS}
            autoComplete="off"
            autoCapitalize="sentences"
            enterKeyHint="enter"
            disabled={voiceBlocked || hasActiveJob || jobPending || startRevision.isPending}
            placeholder={t('revision.placeholder')}
            // The counter is only mounted while the secondary row is open, and a dangling reference
            // is read as no description at all rather than as the one below.
            aria-describedby={expanded ? 'revision-instruction-count' : undefined}
            onChange={(event) => setInstruction(event.target.value)}
            className="max-h-field min-w-0 flex-1"
          />
          <Button
            type="submit"
            variant="secondary"
            size="icon"
            aria-label={t('revision.submit')}
            disabled={disabled}
            pending={startRevision.isPending}
          >
            <SendHorizontal aria-hidden="true" className="size-5" />
          </Button>
        </div>
        {expanded && (
          <div className="grid gap-2">
            {/* The cap used to stop the keystrokes with nothing on screen explaining why. */}
            <Typography variant="meta" as="p" id="revision-instruction-count">
              {instruction.length}/{REVISION_INSTRUCTION_MAX_CHARS}
            </Typography>
            <label
              className={typographyStyles({
                variant: 'label',
                className: 'flex min-h-11 items-center gap-3',
              })}
            >
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
              <Typography variant="body" role="status" className="text-content-secondary">
                {t('revision.ruleLanguageMismatch')}
              </Typography>
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
          </div>
        )}
      </div>
    </form>
  )
})
