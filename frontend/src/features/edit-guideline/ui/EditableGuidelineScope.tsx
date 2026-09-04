import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  GuidelineScopeField,
  canSaveGuideline,
  isOrphanedScope,
  type Guideline,
  type GuidelineScope,
} from '@/entities/guideline'
import { Badge, Button, Editable, FieldMessage, Typography } from '@/shared/ui'

/** A guideline's whole scope, read first and replaced as one patch on save.
 *
 *  The read view is the badge row the list shows anyway: `전역`, the template-name chips, or
 *  `적용 대상 없음` for a scope whose every template was deleted. */
export function EditableGuidelineScope({
  ownerId,
  guideline,
  save,
  errorMessage,
  pending,
  className,
}: {
  ownerId: string
  guideline: Pick<Guideline, 'scope' | 'templates'>
  save: (next: GuidelineScope) => Promise<unknown>
  errorMessage: string
  pending: boolean
  className?: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  return (
    <Editable
      className={className}
      editLabel={t('action.editNamed', {
        ns: 'common',
        name: t('edit.scope', { ns: 'guidelines' }),
      })}
      edit={(exit) => (
        <ScopeEditor
          ownerId={ownerId}
          guideline={guideline}
          save={save}
          errorMessage={errorMessage}
          pending={pending}
          exit={exit}
        />
      )}
    >
      <GuidelineScopeBadges guideline={guideline} />
    </Editable>
  )
}

/** The scope as the list states it. Colour is never the only signal: the orphaned state says so
 *  in words, and its help line explains what to do about it (design-language §9). */
export function GuidelineScopeBadges({
  guideline,
}: {
  guideline: Pick<Guideline, 'scope' | 'templates'>
}) {
  const { t } = useTranslation('guidelines')
  if (isOrphanedScope(guideline)) {
    return (
      <div>
        <Badge tone="warning">{t('scope.orphaned')}</Badge>
        <Typography variant="body" as="p" className="text-content-secondary mt-1">
          {t('scope.orphanedHelp')}
        </Typography>
      </div>
    )
  }
  if (guideline.scope === 'global') {
    return <Badge tone="neutral">{t('scope.global')}</Badge>
  }
  return (
    <div className="flex flex-wrap gap-1">
      {guideline.templates.map((template) => (
        <Badge key={template.id} tone="neutral">
          {template.name}
        </Badge>
      ))}
    </div>
  )
}

function ScopeEditor({
  ownerId,
  guideline,
  save,
  errorMessage,
  pending,
  exit,
}: {
  ownerId: string
  guideline: Pick<Guideline, 'scope' | 'templates'>
  save: (next: GuidelineScope) => Promise<unknown>
  errorMessage: string
  pending: boolean
  exit: () => void
}) {
  const { t } = useTranslation('common')
  // Seeded from the current scope at the mount edit mode gives this editor, and not resynced
  // afterwards — the same rule the text editor follows.
  const [draft, setDraft] = useState<GuidelineScope>({
    kind: guideline.scope,
    templateIds: guideline.templates.map((template) => template.id),
  })
  const [failed, setFailed] = useState(false)

  // The text is not part of this edit, so a non-empty placeholder stands in for it: only the
  // scope half of the shape rule is being checked here.
  const disabled = pending || !canSaveGuideline('x', draft)

  const commit = async () => {
    if (disabled) return
    try {
      await save(draft)
      setFailed(false)
      exit()
    } catch {
      setFailed(true)
    }
  }

  return (
    <div>
      <GuidelineScopeField ownerId={ownerId} value={draft} onChange={setDraft} disabled={pending} />
      {failed && errorMessage && <FieldMessage className="mt-2">{errorMessage}</FieldMessage>}
      <div className="mt-3 flex gap-2">
        <Button onClick={() => void commit()} disabled={disabled} pending={pending}>
          {t('action.save')}
        </Button>
        <Button variant="ghost" onClick={exit} disabled={pending}>
          {t('action.cancel')}
        </Button>
      </div>
    </div>
  )
}
