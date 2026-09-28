import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  GUIDELINE_LIMITS,
  GuidelineScopeField,
  GuidelineTitleField,
  canSaveGuideline,
  guidelineChars,
  remainingGuidelineChars,
  remainingGuidelineTitleChars,
  type Guideline,
  type GuidelineScope,
} from '@/entities/guideline'
import { Button, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'

export interface GuidelinePatch {
  title?: string
  text?: string
  scope?: GuidelineScope
}

/** One guideline's whole edit (GUIDE-47): the title, the text and the scope in one form, saved
 *  together by one 저장 — they are one rule's decision.
 *
 *  The save carries only the parts that differ from the guideline as it was when 수정 was pressed,
 *  so an edit never puts back a part another tab changed; a form that changed nothing leaves
 *  without a call. The scope's shape is checked only when it changed, so an orphaned rule (every
 *  template it named was deleted) can still be renamed. A refused save keeps every typed value. */
export function GuidelineEditForm({
  ownerId,
  guideline,
  save,
  errorMessage,
  pending,
  onDone,
}: {
  ownerId: string
  guideline: Guideline
  save: (patch: GuidelinePatch) => Promise<unknown>
  errorMessage: string
  pending: boolean
  /** Leaves the form: after a save that went through, on 취소, and on a save with no change. */
  onDone: () => void
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const textId = `${id}-text`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  // Seeded once, at the mount 수정 gives this form, and not resynced: while someone is typing
  // here, their draft outranks a value arriving from a refetch.
  const [title, setTitle] = useState(guideline.title)
  const [text, setText] = useState(guideline.text)
  const [scope, setScope] = useState<GuidelineScope>(() => scopeOf(guideline))
  const [failed, setFailed] = useState(false)

  const left = remainingGuidelineChars(text)
  const exceeded = left < 0
  const scopeChanged = !sameScope(scope, scopeOf(guideline))
  const textOk = guidelineChars(text) > 0 && guidelineChars(text) <= GUIDELINE_LIMITS.text
  const disabled =
    pending ||
    !textOk ||
    remainingGuidelineTitleChars(title) < 0 ||
    (scopeChanged && !canSaveGuideline(text, scope))
  const showSaveError = failed && Boolean(errorMessage)

  const commit = async () => {
    if (disabled) return
    const patch: GuidelinePatch = {}
    if (title.trim() !== guideline.title) patch.title = title
    if (text.trim() !== guideline.text) patch.text = text
    if (scopeChanged) patch.scope = scope
    if (Object.keys(patch).length === 0) {
      onDone()
      return
    }
    try {
      await save(patch)
      setFailed(false)
      onDone()
    } catch {
      // Stay open: the message renders below and every typed value is still here to fix.
      setFailed(true)
    }
  }

  return (
    <div>
      <GuidelineTitleField id={`${id}-name`} value={title} onChange={setTitle} disabled={pending} />
      <FieldLabel htmlFor={textId} className="mt-4">
        {t('edit.text', { ns: 'guidelines' })}
      </FieldLabel>
      <Textarea
        id={textId}
        value={text}
        onChange={(event) => setText(event.target.value)}
        rows={2}
        autoGrow
        autoFocus
        disabled={pending}
        aria-invalid={exceeded || failed || undefined}
        aria-describedby={`${countId}${showSaveError ? ` ${errorId}` : ''}`}
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
      <GuidelineScopeField
        ownerId={ownerId}
        kind={guideline.kind}
        value={scope}
        onChange={setScope}
        disabled={pending}
        className="mt-4"
      />
      {showSaveError && (
        <FieldMessage id={errorId} className="mt-3">
          {errorMessage}
        </FieldMessage>
      )}
      <div className="mt-3 flex gap-2">
        <Button onClick={() => void commit()} disabled={disabled} pending={pending}>
          {t('action.save', { ns: 'common' })}
        </Button>
        <Button variant="ghost" onClick={onDone} disabled={pending}>
          {t('action.cancel', { ns: 'common' })}
        </Button>
      </div>
    </div>
  )
}

function scopeOf(guideline: Pick<Guideline, 'scope' | 'templates' | 'fields'>): GuidelineScope {
  return {
    kind: guideline.scope,
    templateIds: guideline.templates.map((template) => template.id),
    fields: guideline.fields,
  }
}

/** A scope is its kind plus its sets; the order the sets were picked in is not part of it. */
function sameScope(a: GuidelineScope, b: GuidelineScope): boolean {
  const same = (x: readonly string[], y: readonly string[]) =>
    x.length === y.length && [...x].sort().every((value, at) => value === [...y].sort()[at])
  return a.kind === b.kind && same(a.templateIds, b.templateIds) && same(a.fields, b.fields)
}
