import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { useTemplates } from '@/entities/template/@x/guideline'
import { Checkbox, FieldLabel, SegmentedControl, Typography } from '@/shared/ui'
import type { GuidelineScope, GuidelineScopeKind } from '../model/types'
import { GuidelineFieldPicker } from './GuidelineFieldPicker'

/** The scope editor, shared by the create form and the whole-scope edit.
 *
 *  It lives with the entity rather than in either action slice because both need the identical
 *  control and a feature may not import a sibling feature. It edits a scope VALUE and performs no
 *  mutation of its own — each caller owns the save.
 *
 *  Switching kind clears the other kind's set instead of remembering it: every shape the server
 *  refuses mixes two kinds' sets or leaves a narrowed kind with none (GUIDE-5), so the control can
 *  never hold a mixed one. The pickers are checkbox lists rather than a native multi-select — a
 *  multiple-choice select needs ctrl-click on a desktop and is close to unusable on a phone
 *  (design-language §1.1).
 */
export function GuidelineScopeField({
  ownerId,
  value,
  onChange,
  disabled = false,
  className,
}: {
  ownerId: string
  value: GuidelineScope
  onChange: (next: GuidelineScope) => void
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('guidelines')
  const id = useId()
  const { templates } = useTemplates(ownerId)

  const setKind = (kind: GuidelineScopeKind) => {
    switch (kind) {
      case 'global':
        return onChange({ kind, templateIds: [], fields: [] })
      case 'templates':
        return onChange({ kind, templateIds: value.templateIds, fields: [] })
      case 'fields':
        return onChange({ kind, templateIds: [], fields: value.fields })
    }
  }
  const toggle = (templateId: string) => {
    const next = value.templateIds.includes(templateId)
      ? value.templateIds.filter((current) => current !== templateId)
      : [...value.templateIds, templateId]
    onChange({ kind: 'templates', templateIds: next, fields: [] })
  }
  const help = {
    global: t('scope.globalHelp'),
    templates: t('scope.templatesHelp'),
    fields: t('scope.fieldsHelp'),
  }[value.kind]

  return (
    <div className={className}>
      <Typography variant="label" as="p">
        {t('scope.label')}
      </Typography>
      <SegmentedControl
        value={value.kind}
        options={[
          { value: 'global', label: t('scope.global') },
          { value: 'templates', label: t('scope.templates') },
          { value: 'fields', label: t('scope.fields') },
        ]}
        onChange={setKind}
        ariaLabel={t('scope.label')}
        className="mt-2"
      />
      <Typography variant="body" as="p" className="text-content-secondary mt-2">
        {help}
      </Typography>

      {value.kind === 'templates' && (
        <fieldset className="mt-3" disabled={disabled}>
          <legend className="sr-only">{t('scope.pick')}</legend>
          {templates.length === 0 ? (
            <Typography variant="body" as="p" className="text-content-tertiary">
              {t('scope.templatesEmpty')}
            </Typography>
          ) : (
            <ul className="space-y-1">
              {templates.map((template) => (
                <li key={template.id} className="flex items-center gap-3">
                  <Checkbox
                    id={`${id}-${template.id}`}
                    checked={value.templateIds.includes(template.id)}
                    onChange={() => toggle(template.id)}
                  />
                  <FieldLabel htmlFor={`${id}-${template.id}`}>{template.name}</FieldLabel>
                </li>
              ))}
            </ul>
          )}
        </fieldset>
      )}

      {value.kind === 'fields' && (
        <GuidelineFieldPicker
          value={value.fields}
          onChange={(fields) => onChange({ kind: 'fields', templateIds: [], fields })}
          legend={t('scope.pickFields')}
          disabled={disabled}
          className="mt-3"
        />
      )}
    </div>
  )
}
