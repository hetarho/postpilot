import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipComposition } from '@/entities/clip-template/@x/clip-project'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import { Button, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'
import type { ClipCompositionInputs as Inputs } from '../model/composition'
import { removeCompositionItem } from '../model/composition-inputs'

export function ClipCompositionInputFields({
  document,
  value,
  onChange,
}: {
  document: ClipComposition
  value: Inputs
  onChange: (value: Inputs) => void
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const fields = (
    group: string,
    values: Record<string, string>,
    key: string,
    change: (values: Record<string, string>) => void,
  ) =>
    document.fields
      .filter((f) => f.group === group)
      .map((f) => {
        const fieldId = `${id}-${key}-${f.id}`
        const text = values[f.id] ?? ''
        const missing = f.required && !text.trim()
        const tooLong = Array.from(text).length > CLIP_COMPOSITION_LIMITS.answerChars
        return (
          <div key={f.id} className="min-w-0 space-y-2">
            <FieldLabel htmlFor={fieldId}>
              {f.label}{' '}
              {t(f.required ? 'composition.requiredSuffix' : 'composition.optionalSuffix')}
            </FieldLabel>
            {f.prompt && (
              <Typography variant="body" className="text-content-secondary break-words">
                {f.prompt}
              </Typography>
            )}
            <Textarea
              id={fieldId}
              aria-label={f.label}
              aria-required={f.required}
              value={text}
              autoGrow
              aria-invalid={missing || tooLong}
              onChange={(e) => change({ ...values, [f.id]: e.target.value })}
            />
            {missing && <FieldMessage>{t('validation.required')}</FieldMessage>}
            {tooLong && (
              <FieldMessage>
                {t('validation.tooLong', { max: CLIP_COMPOSITION_LIMITS.answerChars })}
              </FieldMessage>
            )}
          </div>
        )
      })
  return (
    <div className="min-w-0 space-y-6">
      {fields('', value.values, 'global', (values) => onChange({ ...value, values }))}
      {document.groups.map((group, n) => (
        <section
          key={group}
          className="min-w-0 space-y-4"
          aria-label={t('composition.groupNumber', { n: n + 1 })}
        >
          <Typography variant="fieldTitle" as="h2">
            {t('composition.groupNumber', { n: n + 1 })}
          </Typography>
          <Typography variant="body" className="text-content-secondary">
            {t('composition.itemsHelp')}
          </Typography>
          {(value.items[group] ?? []).map((item, i) => (
            <fieldset key={item.id} className="min-w-0 space-y-4">
              <Typography variant="label" as="legend">
                {t('composition.itemNumber', { n: i + 1 })}
              </Typography>
              {fields(group, item.values, `${group}-${item.id}`, (values) =>
                onChange({
                  ...value,
                  items: {
                    ...value.items,
                    [group]: value.items[group].map((other) =>
                      other.id === item.id ? { ...item, values } : other,
                    ),
                  },
                }),
              )}
              <Button
                variant="ghost"
                onClick={() => onChange(removeCompositionItem(value, group, item.id))}
              >
                {t('composition.removeItem', { n: i + 1 })}
              </Button>
            </fieldset>
          ))}
          <Button
            variant="ghost"
            disabled={(value.items[group]?.length ?? 0) >= CLIP_COMPOSITION_LIMITS.items}
            onClick={() =>
              onChange({
                ...value,
                items: {
                  ...value.items,
                  [group]: [...(value.items[group] ?? []), { id: crypto.randomUUID(), values: {} }],
                },
              })
            }
          >
            {t('composition.addItem')}
          </Button>
        </section>
      ))}
    </div>
  )
}
