import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { remainingChars, TEMPLATE_LIMITS } from '@/entities/template'
import { FieldCount, FieldLabel, Switch, Textarea, Toggletip, Typography } from '@/shared/ui'
import type { AnswerField } from '../model/answers'

/** The selected template's data fields, under ①'s memo (POST-54, POST-62).
 *
 *  They belong to ① for the same reason the memo does: they are the material the next run is
 *  written from. A field switched off stays visible, greyed, with its text kept and readable —
 *  the switch is how the author says "I have nothing for this", and losing what was typed would
 *  make trying it twice expensive.
 *
 *  Nothing renders at all when the post has no template, when its template declares no field, or
 *  when its body does not parse: an empty section would be a question with no question in it. */
export function TemplateAnswerFields({
  fields,
  disabled = false,
  onChange,
}: {
  fields: readonly AnswerField[]
  disabled?: boolean
  onChange: (label: string, change: { text?: string; enabled?: boolean }) => void
}) {
  const { t } = useTranslation('posts')
  const id = useId()

  if (fields.length === 0) return null

  return (
    <section aria-labelledby={`${id}-heading`} className="mt-6">
      {/* What the fields are for is behind the ⓘ rather than a standing line under the heading
          (owner decision 2026-09-25). */}
      <div className="flex items-center gap-1">
        <Typography variant="fieldTitle" as="h3" id={`${id}-heading`}>
          {t('editor.answers.heading')}
        </Typography>
        <Toggletip label={t('editor.answers.explain')} className="-my-2">
          {t('editor.answers.help')}
        </Toggletip>
      </div>

      {fields.map((field) => {
        const fieldId = `${id}-${field.label}`
        return (
          <div key={field.label} className="mt-4">
            <div className="flex min-h-11 items-center gap-3">
              <FieldLabel htmlFor={fieldId} className="min-w-0 flex-1">
                {field.label}
              </FieldLabel>
              <Switch
                aria-label={t('editor.answers.include', { title: field.label })}
                checked={field.enabled}
                disabled={disabled}
                onChange={(event) => onChange(field.label, { enabled: event.target.checked })}
              />
            </div>
            <Textarea
              id={fieldId}
              value={field.text}
              // Off is not empty: the text stays readable, which is what makes the switch a
              // decision the author can take back.
              disabled={disabled || !field.enabled}
              rows={2}
              autoGrow
              enterKeyHint="enter"
              onChange={(event) => onChange(field.label, { text: event.target.value })}
              className="mt-1"
            />
            <FieldCount left={remainingChars(field.text, TEMPLATE_LIMITS.askValue)} />
            {!field.enabled && (
              <Typography variant="meta" as="p">
                {t('editor.answers.excluded')}
              </Typography>
            )}
          </div>
        )
      })}
    </section>
  )
}
