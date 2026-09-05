import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  canSaveGuideline,
  globalScope,
  isDuplicateGuideline,
  remainingGuidelineChars,
  useCreateGuidelineCall,
  type GuidelineScope,
} from '@/entities/guideline'
import {
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  Textarea,
  Typography,
} from '@/shared/ui'

/** Turns a revision instruction into a saved guideline, beside the pre-flight `규칙으로 저장`.
 *
 *  It is an explicit user save of user-authored text, not learning: the dialog seeds the
 *  instruction and the user edits it before saving (a raw "무인 매장이니까 주인 얘기 빼줘" is
 *  usually generalized first), and no model or job can reach this path (plan 16 non-goals).
 *
 *  The template option comes from the ALREADY-LOADED post, so opening this dialog issues no query
 *  and starts nothing ([I5]).
 *
 *  Unchanged by change 26, and deliberately so: this stays the immediate path for a user who
 *  already knows the rule. Saving here also approves the candidate the completed revision
 *  recorded — the server matches it by text in the create's own transaction — which is why the
 *  instruction saved from this dialog never afterwards appears in the 후보 section. */
export function SaveAsGuidelineButton({
  ownerId,
  instruction,
  template,
  disabled = false,
}: {
  ownerId: string
  /** The instruction the completed revision ran with; it seeds the dialog. */
  instruction: string
  /** The post's current template, or undefined when it has none — then only 전역 is offered. */
  template?: { id: string; name: string }
  disabled?: boolean
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const fieldId = `${id}-text`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const [open, setOpen] = useState(false)
  const [text, setText] = useState(instruction)
  const [scope, setScope] = useState<GuidelineScope>(globalScope)
  const [saved, setSaved] = useState(false)
  const create = useCreateGuidelineCall(ownerId)

  // Seeded on OPEN rather than from an effect on `instruction`: a refetch or a second revision
  // must not overwrite what someone is editing here, and reopening starts from the current
  // instruction rather than from the previous attempt's draft.
  const openDialog = () => {
    setText(instruction)
    setScope(globalScope())
    setSaved(false)
    setOpen(true)
  }

  const left = remainingGuidelineChars(text)
  const exceeded = left < 0
  const showCreateError = create.isError && !isDuplicateGuideline(create.error)
  const blocked = !canSaveGuideline(text, scope) || create.isPending

  const confirm = async () => {
    if (blocked) return
    try {
      await create.create(text, scope)
      setSaved(true)
      setOpen(false)
    } catch (cause) {
      // An exact duplicate is information, not a failure: the rule the user wanted is already
      // saved, so the dialog closes and says so.
      if (isDuplicateGuideline(cause)) {
        setSaved(true)
        setOpen(false)
        return
      }
      // Any other refusal keeps the dialog open with the draft intact.
    }
  }

  return (
    <>
      <Button variant="ghost" disabled={disabled} onClick={openDialog}>
        {t('capture.action', { ns: 'guidelines' })}
      </Button>
      {saved && !open && (
        <Typography variant="body" role="status" className="text-content-secondary">
          {isDuplicateGuideline(create.error)
            ? t('capture.duplicate', { ns: 'guidelines' })
            : t('capture.saved', { ns: 'guidelines' })}
        </Typography>
      )}
      <Dialog
        open={open}
        title={t('capture.title', { ns: 'guidelines' })}
        confirmLabel={t('capture.submit', { ns: 'guidelines' })}
        pending={create.isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void confirm()}
      >
        <Typography variant="body" className="text-content-secondary">
          {t('capture.description', { ns: 'guidelines' })}
        </Typography>
        <FieldLabel htmlFor={fieldId} className="mt-4 block">
          {t('create.text', { ns: 'guidelines' })}
        </FieldLabel>
        <Textarea
          id={fieldId}
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={3}
          autoGrow
          aria-invalid={exceeded || showCreateError || undefined}
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
        {/* Only two options and only when the post has a template — the whole directory belongs on
            /guidelines, not in a dialog opened mid-revision. */}
        {template && (
          <>
            <Typography variant="label" as="p" className="mt-4">
              {t('scope.label', { ns: 'guidelines' })}
            </Typography>
            <SegmentedControl
              value={scope.kind}
              options={[
                { value: 'global', label: t('capture.scopeGlobal', { ns: 'guidelines' }) },
                {
                  value: 'templates',
                  label: t('capture.scopeTemplate', { ns: 'guidelines', name: template.name }),
                },
              ]}
              onChange={(kind) =>
                setScope(kind === 'global' ? globalScope() : { kind, templateIds: [template.id] })
              }
              ariaLabel={t('scope.label', { ns: 'guidelines' })}
              className="mt-2"
            />
          </>
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
