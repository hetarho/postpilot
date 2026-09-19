import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipFieldMaximum,
  compositionCharacters,
  type ClipComposition,
} from '@/entities/clip-template/@x/clip-project'
import { CLIP_DEFAULT_REGION_PRESETS } from '../config'
import { type ClipRegionPresets } from '../config'
import { Button, FieldCount, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'
import type { ClipCompositionInputs as Inputs } from '../model/composition'
import { compositionInputsAtMinimum, removeCompositionItem } from '../model/composition-inputs'
import { boundedText } from '../lib/bounded-text'

export function ClipCompositionInputFields({
  document,
  presets = CLIP_DEFAULT_REGION_PRESETS,
  value: stored,
  onChange,
}: {
  document: ClipComposition
  /** The project's own presets, which decide how long an answer bound into an
   *  intro or outro line may be (CLIP-117, CLIP-147). */
  presets?: ClipRegionPresets
  value: Inputs
  onChange: (value: Inputs) => void
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const value = compositionInputsAtMinimum(document, stored)
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
        // The maximum every position this answer reaches agrees on: the
        // parser's, narrowed by the region slots the project's own presets put
        // this answer in (CLIP-117). A field that reaches no bounded position
        // still has the grammar's own ceiling, so no field loses its counter.
        const max = clipFieldMaximum(document, presets, f.group ? `${f.group}.${f.id}` : f.id)
        const count = compositionCharacters(text)
        // Only an answer stored before its template tightened this number can be
        // over it: nothing typed here gets past the bound.
        const tooLong = count > max
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
              onChange={(e) => change({ ...values, [f.id]: boundedText(e.target.value, max) })}
            />
            <FieldCount left={max - count} />
            {missing && <FieldMessage>{t('validation.required')}</FieldMessage>}
            {tooLong && <FieldMessage>{t('validation.tooLong', { max })}</FieldMessage>}
          </div>
        )
      })
  return (
    <div className="min-w-0 space-y-6">
      {fields('', value.values, 'global', (values) => onChange({ ...value, values }))}
      {document.groups.map(({ id: group, label, min, max }, n) => {
        // The number the group actually admits (CLIP-119), so an item carrying
        // a required field cannot be removed down to nothing.
        const minimum = document.minima[group] ?? min
        return (
          <section
            key={group}
            className="min-w-0 space-y-4"
            aria-label={label.trim() || t('composition.groupNumber', { n: n + 1 })}
          >
            <Typography variant="fieldTitle" as="h2">
              {label.trim() || t('composition.groupNumber', { n: n + 1 })}
            </Typography>
            <Typography variant="body" className="text-content-secondary">
              {t('composition.itemsHelp')}
            </Typography>
            {(value.items[group] ?? []).map((item, i) => (
              <fieldset key={item.id} className="min-w-0 space-y-4">
                <Typography variant="label" as="legend">
                  {label.trim()
                    ? t('composition.namedItemNumber', { name: label, n: i + 1 })
                    : t('composition.itemNumber', { n: i + 1 })}
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
                {value.items[group].length > minimum && (
                  <Button
                    variant="ghost"
                    onClick={() => onChange(removeCompositionItem(value, group, item.id))}
                  >
                    {label.trim()
                      ? t('composition.removeNamedItem', { name: label, n: i + 1 })
                      : t('composition.removeItem', { n: i + 1 })}
                  </Button>
                )}
              </fieldset>
            ))}
            <Button
              variant="ghost"
              disabled={(value.items[group]?.length ?? 0) >= max}
              onClick={() =>
                onChange({
                  ...value,
                  items: {
                    ...value.items,
                    [group]: [
                      ...(value.items[group] ?? []),
                      { id: crypto.randomUUID(), values: {} },
                    ],
                  },
                })
              }
            >
              {label.trim()
                ? t('composition.addNamedItem', { name: label })
                : t('composition.addItem')}
            </Button>
          </section>
        )
      })}
    </div>
  )
}
