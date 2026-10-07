import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { AuthoringArtifact } from '@/entities/ai-authoring'
import { isBlogFieldId } from '@/entities/blog-field'
import {
  GuidelineScopeField,
  GuidelineTitleField,
  remainingGuidelineChars,
  type GuidelineKind,
  type GuidelineScope,
} from '@/entities/guideline'
import { FieldCount, FieldLabel, Textarea } from '@/shared/ui'

export function GuidelineDirectEditor({
  ownerId,
  kind,
  source,
  onChange,
  disabled,
}: {
  ownerId: string
  kind: GuidelineKind
  source: AuthoringArtifact
  onChange: (source: AuthoringArtifact) => void
  disabled: boolean
}) {
  const { t } = useTranslation('guidelines')
  const id = useId()
  const scope: GuidelineScope = {
    kind: source.scope === 'templates' || source.scope === 'fields' ? source.scope : 'global',
    templateIds: source.templateIds ?? [],
    fields: (source.fields ?? []).filter(isBlogFieldId),
  }
  return (
    <div className="space-y-6">
      <GuidelineTitleField
        id={id + '-name'}
        value={source.name}
        onChange={(name) => onChange({ ...source, name })}
        disabled={disabled}
      />
      <div>
        <FieldLabel htmlFor={id + '-text'}>{t('edit.text')}</FieldLabel>
        <Textarea
          id={id + '-text'}
          value={source.body}
          rows={3}
          autoGrow
          disabled={disabled}
          aria-invalid={remainingGuidelineChars(source.body) < 0 || undefined}
          onChange={(e) => onChange({ ...source, body: e.target.value })}
          className="mt-1"
        />
        <FieldCount left={remainingGuidelineChars(source.body)} />
      </div>
      <GuidelineScopeField
        ownerId={ownerId}
        kind={kind}
        value={scope}
        disabled={disabled}
        onChange={(value) =>
          onChange({
            ...source,
            scope: value.kind,
            templateIds: value.templateIds,
            fields: value.fields,
          })
        }
      />
    </div>
  )
}
