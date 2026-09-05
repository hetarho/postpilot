import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  GuidelineScopeField,
  canSaveGuideline,
  globalScope,
  invalidateGuidelineCandidates,
  isDuplicateGuideline,
  remainingGuidelineChars,
  useCreateGuidelineCall,
  type GuidelineCandidate,
  type GuidelineScope,
} from '@/entities/guideline'
import { Button, Dialog, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'

/** 승인 — turns a recorded candidate into a saved guideline through the standard create.
 *
 *  There is no Approve procedure: the create already owns the text bound, the account cap and
 *  every refusal an approval needs, so this is the same call the create form makes (change 26).
 *  The candidate is marked approved server-side in the same transaction.
 *
 *  Scope is asked HERE and nowhere earlier. A candidate carries none — that is what lets recording
 *  be automatic — so nothing is ever applied to a post without the user picking, at this moment,
 *  what it applies to. 전역 is preselected because a rule is meant to apply everywhere unless it
 *  is narrowed.
 *
 *  The text opens editable with the live count because a candidate is stored at the REVISION bound
 *  (500) and a guideline's is 300: an over-long candidate is shortened here rather than truncated
 *  anywhere. The server still refuses one that is saved unedited, and that refusal renders below. */
export function ApproveGuidelineCandidateButton({
  ownerId,
  candidate,
  disabled = false,
}: {
  ownerId: string
  candidate: GuidelineCandidate
  disabled?: boolean
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const fieldId = `${id}-text`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const [open, setOpen] = useState(false)
  const [text, setText] = useState(candidate.text)
  const [scope, setScope] = useState<GuidelineScope>(globalScope)
  const [duplicate, setDuplicate] = useState(false)
  const create = useCreateGuidelineCall(ownerId)
  const queryClient = useQueryClient()
  const transport = useTransport()

  // Seeded on OPEN, not from an effect on the candidate: a refetch that bumps this candidate's
  // occurrence count must not overwrite what someone is editing, and reopening starts from the
  // recorded text rather than from the previous attempt's draft.
  const openDialog = () => {
    setText(candidate.text)
    setScope(globalScope())
    setDuplicate(false)
    // The previous attempt's refusal goes with its draft. Without this, reopening shows the
    // old "too long" or cap message under text that no longer provoked it.
    create.reset()
    setOpen(true)
  }

  const left = remainingGuidelineChars(text)
  const exceeded = left < 0
  const showCreateError = create.isError && !isDuplicateGuideline(create.error)
  const blocked = !canSaveGuideline(text, scope) || create.isPending

  const confirm = async () => {
    if (blocked) return
    try {
      // The id names the row this approval approves, rather than leaving it to the server's
      // text match — which cannot find it once the text has been edited here.
      await create.create(text, scope, candidate.id)
      setOpen(false)
    } catch (cause) {
      // An exact duplicate means this text is already a saved guideline, so no create can
      // succeed with it. The list is re-read because the usual way to reach this is another tab
      // having just saved it — which also approved this candidate, so the row may already be
      // gone. If it is still here, the message says why, and 무시 is the way out.
      if (isDuplicateGuideline(cause)) {
        setDuplicate(true)
        invalidateGuidelineCandidates(queryClient, transport, ownerId)
        return
      }
      // Any other refusal — over the text bound, past the account cap — keeps the dialog open
      // with the draft intact so it can be fixed.
    }
  }

  return (
    <>
      <Button disabled={disabled} onClick={openDialog}>
        {t('candidate.approve', { ns: 'guidelines' })}
      </Button>
      <Dialog
        open={open}
        title={t('candidate.approveTitle', { ns: 'guidelines' })}
        confirmLabel={t('candidate.approveSubmit', { ns: 'guidelines' })}
        pending={create.isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void confirm()}
      >
        <Typography variant="body" className="text-content-secondary">
          {t('candidate.approveDescription', { ns: 'guidelines' })}
        </Typography>
        <FieldLabel htmlFor={fieldId} className="mt-4 block">
          {t('create.text', { ns: 'guidelines' })}
        </FieldLabel>
        <Textarea
          id={fieldId}
          value={text}
          onChange={(event) => {
            setText(event.target.value)
            setDuplicate(false)
          }}
          rows={3}
          autoGrow
          aria-invalid={exceeded || duplicate || showCreateError || undefined}
          aria-describedby={`${countId}${showCreateError ? ` ${errorId}` : ''}`}
          className="mt-1"
        />
        {exceeded ? (
          <FieldMessage id={countId} role="status" className="mt-2">
            {t('count.exceeded', { ns: 'common', count: -left })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p" id={countId} className="mt-2">
            {t('count.remaining', { ns: 'common', count: left })}
          </Typography>
        )}
        {/* The entity's own scope control, not a second one: the create form and this dialog must
            offer the same scopes, and the whole template directory belongs in both. */}
        <GuidelineScopeField
          ownerId={ownerId}
          value={scope}
          onChange={setScope}
          disabled={create.isPending}
          className="mt-6"
        />
        {duplicate && (
          <FieldMessage role="status" className="mt-3">
            {t('candidate.approveDuplicate', { ns: 'guidelines' })}
          </FieldMessage>
        )}
        {showCreateError && (
          <FieldMessage id={errorId} className="mt-3">
            {create.errorMessage}
          </FieldMessage>
        )}
      </Dialog>
    </>
  )
}
