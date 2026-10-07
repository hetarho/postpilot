import { forwardRef, useCallback, useImperativeHandle, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { SendHorizontal } from 'lucide-react'
import { isTerminal, useStartRevision, type GenerationJob } from '@/entities/generation-job'
import { useSelectionSavePending, useStageSelection } from '@/entities/model-catalog'
import { ContentRevisionConflictError } from '@/entities/post'
import { voiceAIRefusal, type VoiceRef } from '@/entities/voice'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { REVISION_INSTRUCTION_MAX_CHARS } from '../config'
import { AppFailureMessage, Button, FieldMessage, Notice, Textarea, Typography } from '@/shared/ui'

import { SaveAsGuidelineButton } from './SaveAsGuidelineButton'

interface ReviseFormProps {
  ownerId: string
  postSlug: string
  /** The post's voice, undefined for 말투 없음: a deleted or not-yet-made one refuses revision
   *  before any provider call (POST-25). */
  voice: Pick<VoiceRef, 'deleted' | 'made'> | undefined
  /** The post's current template, read from the already-loaded post so the guideline capture can
   *  offer it as a scope without issuing a query. Empty id means the post has none. */
  template?: { id: string; name: string }
  activeJob?: GenerationJob
  jobPending?: boolean
  onStarted: (jobId: string) => void
  beforeStart?: () => Promise<void>
  /** Rendered at the top-right of the row's own heading. `widgets/refine-dock` puts 확정하기 there:
   *  the step's way OUT belongs beside the name of the loop it leaves, not underneath the field
   *  that continues it. A slot rather than an import, because a feature may not reach a sibling
   *  feature (ARCH-13). */
  action?: ReactNode
}

export interface ReviseFormHandle {
  start: () => void
}

/** Replaces the current canonical content through one durable `revise` job.
 *
 *  It is the body of 글 다듬기's dock (`widgets/refine-dock`) and renders no surface of its own: a
 *  4,000px draft used to put this form past the end of the page, which is exactly where THEME-24 says
 *  a committing action may not live. It DOES render the row's heading, with the step's way out
 *  (확정하기) in the `action` slot beside it.
 *
 *  Its SECONDARY controls — the counter and 지침으로 저장 — collapse while the field is empty and
 *  unfocused. The dock is over the draft the whole time, so the row that is not being used is
 *  height taken from the thing the screen is for (THEME-8). They come back on focus, on the first
 *  character, and for as long as a revision is running or has failed, because that is when their
 *  state is worth reading. */
export const ReviseForm = forwardRef<ReviseFormHandle, ReviseFormProps>(function ReviseForm(
  {
    ownerId,
    postSlug,
    voice,
    template,
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
  const [focused, setFocused] = useState(false)
  const [prepareFailure, setPrepareFailure] = useState<AppFailure | 'content-conflict'>()
  const write = useStageSelection('write')
  const selectionSaving = useSelectionSavePending()
  const startRevision = useStartRevision()
  const hasActiveJob = Boolean(activeJob && !isTerminal(activeJob))
  // A completed REVISION, not just any completed job: a finished `generate` job leaves the
  // instruction box holding text that never ran, and 'done' rather than merely terminal because a
  // failed revision produced nothing worth turning into a guideline.
  const revisionCompleted = activeJob?.kind === 'revise' && activeJob.status === 'done'
  const revisionBusy =
    activeJob?.kind === 'revise' && (!isTerminal(activeJob) || activeJob.status === 'failed')
  const voiceRefusal = voiceAIRefusal(voice)
  const voiceBlocked = Boolean(voiceRefusal)
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
      const response = await startRevision.start(postSlug, trimmed, write.selected)
      onStarted(response.jobId)
    } catch {
      // The mutation owns and renders the transport/provider error.
    }
  }, [beforeStart, disabled, onStarted, postSlug, startRevision, trimmed, write.selected])

  useImperativeHandle(ref, () => ({ start: () => void start() }), [start])

  const blocker = voiceBlocked
    ? voiceRefusal
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
          (THEME-19). Keep the step action at its natural label width; when it cannot fit beside
          the field label, the heading row wraps without splitting its button into characters. */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Typography
          variant="fieldTitle"
          as="label"
          htmlFor="revision-instruction"
          className="min-w-0"
        >
          {t('revision.instruction')}
        </Typography>
        {action && <div className="min-w-0 shrink-0">{action}</div>}
      </div>
      {/* Validation and failure sit ABOVE the controls, so the keyboard covering the bottom ~40%
          of the screen hides at most a button and never the reason it is disabled (THEME-31). */}
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
          // Only a focus move OUT of this group collapses the row: tabbing from the field onto
          // 지침으로 저장 inside it must not unmount the button mid-gesture.
          if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false)
        }}
      >
        <div className="flex items-end gap-2">
          {/* A textarea, not a single-line field: at 360px one line shows ~20 of the 500 permitted
            Hangul, so an ordinary instruction scrolled its own beginning out of sight while it was
            being typed. `autoGrow` keeps it out of the page's scroll (THEME-25); Return inserts a line
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
            {/* Only after a revision has actually finished: the instruction is worth keeping as a
              guideline once the user has seen what it did, and a guideline is a plain create, so it
              can wait for the result. */}
            {revisionCompleted && (
              <div className="flex flex-wrap items-center gap-2">
                <SaveAsGuidelineButton
                  ownerId={ownerId}
                  instruction={trimmed}
                  template={template?.id ? template : undefined}
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
