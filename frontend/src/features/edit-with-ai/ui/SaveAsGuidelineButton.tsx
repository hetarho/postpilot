import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  canSaveGuideline,
  globalScope,
  isDuplicateGuideline,
  remainingGuidelineChars,
  useCreateGuidelineCall,
  type GuidelineScope,
} from '@/entities/guideline'
import { Button, Dialog, FieldLabel, FieldMessage, SegmentedControl, Textarea } from '@/shared/ui'

/** Turns a revision instruction into a saved guideline, beside the pre-flight `규칙으로 저장`.
 *
 *  It is an explicit user save of user-authored text, not learning: the dialog seeds the
 *  instruction and the user edits it before saving (a raw "무인 매장이니까 주인 얘기 빼줘" is
 *  usually generalized first), and no model or job can reach this path (plan 16 non-goals).
 *
 *  The purpose option comes from the ALREADY-LOADED post, so opening this dialog issues no query
 *  and starts nothing ([I5]). */
export function SaveAsGuidelineButton({
  ownerId,
  instruction,
  purpose,
  disabled = false,
}: {
  ownerId: string
  /** The instruction the completed revision ran with; it seeds the dialog. */
  instruction: string
  /** The post's current purpose, or undefined when it has none — then only 전역 is offered. */
  purpose?: { id: string; name: string }
  disabled?: boolean
}) {
  const { t } = useTranslation(['guidelines', 'common'])
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
        <p role="status" className="text-content-secondary text-sm">
          {isDuplicateGuideline(create.error)
            ? t('capture.duplicate', { ns: 'guidelines' })
            : t('capture.saved', { ns: 'guidelines' })}
        </p>
      )}
      <Dialog
        open={open}
        title={t('capture.title', { ns: 'guidelines' })}
        confirmLabel={t('capture.submit', { ns: 'guidelines' })}
        pending={create.isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void confirm()}
      >
        <p className="text-content-secondary text-sm leading-relaxed">
          {t('capture.description', { ns: 'guidelines' })}
        </p>
        <FieldLabel htmlFor="guideline-capture-text" className="mt-4 block">
          {t('create.text', { ns: 'guidelines' })}
        </FieldLabel>
        <Textarea
          id="guideline-capture-text"
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={3}
          autoGrow
          className="mt-1"
        />
        <p
          className={
            left < 0 ? 'text-field-error mt-2 text-xs' : 'text-content-tertiary mt-2 text-xs'
          }
        >
          {left < 0
            ? t('count.exceeded', { ns: 'common', count: -left })
            : t('count.remaining', { ns: 'common', count: left })}
        </p>
        {/* Only two options and only when the post has a purpose — the whole directory belongs on
            /guidelines, not in a dialog opened mid-revision. */}
        {purpose && (
          <>
            <p className="text-content-tertiary mt-4 text-xs font-medium">
              {t('scope.label', { ns: 'guidelines' })}
            </p>
            <SegmentedControl
              value={scope.kind}
              options={[
                { value: 'global', label: t('capture.scopeGlobal', { ns: 'guidelines' }) },
                {
                  value: 'purposes',
                  label: t('capture.scopePurpose', { ns: 'guidelines', name: purpose.name }),
                },
              ]}
              onChange={(kind) =>
                setScope(kind === 'global' ? globalScope() : { kind, purposeIds: [purpose.id] })
              }
              ariaLabel={t('scope.label', { ns: 'guidelines' })}
              className="mt-2"
            />
          </>
        )}
        {create.isError && !isDuplicateGuideline(create.error) && (
          <FieldMessage className="mt-3">{create.errorMessage}</FieldMessage>
        )}
      </Dialog>
    </>
  )
}
