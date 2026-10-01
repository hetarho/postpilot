import { useId, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { isBlank, remainingChars, TEMPLATE_LIMITS } from '@/entities/template'
import {
  Button,
  FieldCount,
  FieldLabel,
  FieldMessage,
  Switch,
  Textarea,
  Toggletip,
  Typography,
} from '@/shared/ui'
import { firstEnabledAnswer, missingRequiredAnswers, type AnswerField } from '../model/answers'

/** The selected template's data fields, between ①'s 가제 and its memo (POST-54, POST-62).
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
  showRequiredErrors = false,
  onChange,
  firstFieldRef,
}: {
  fields: readonly AnswerField[]
  disabled?: boolean
  showRequiredErrors?: boolean
  onChange: (label: string, change: { text?: string; enabled?: boolean }) => void
  /** Attached to the first field that takes typing, so the 가제's Enter lands on the next field on
   *  screen (`firstEnabledAnswer`). */
  firstFieldRef?: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  const id = useId()
  const first = firstEnabledAnswer(fields)
  const missing = new Set(missingRequiredAnswers(fields))

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
      {showRequiredErrors && missing.size > 0 && (
        <FieldMessage className="mt-2">
          {t('editor.answers.requiredSummary', { fields: [...missing].join(', ') })}
        </FieldMessage>
      )}

      {fields.map((field, index) => {
        const fieldId = `${id}-${index}`
        const needsAnswer = missing.has(field.label)
        const showError = showRequiredErrors && needsAnswer
        const describedBy = [
          field.prompt ? `${fieldId}-help` : '',
          showError ? `${fieldId}-error` : '',
        ]
          .filter(Boolean)
          .join(' ')
        return (
          <div key={field.label} className="mt-4">
            <div className="flex min-h-11 items-center gap-3">
              <FieldLabel htmlFor={fieldId} className="min-w-0 flex-1">
                {field.label}
                {field.required && (
                  <Typography as="span" variant="meta" className="text-content-secondary ml-2">
                    {t('editor.answers.required')}
                  </Typography>
                )}
              </FieldLabel>
              {!field.required && (
                <Switch
                  aria-label={t('editor.answers.include', { title: field.label })}
                  checked={field.enabled}
                  disabled={disabled}
                  onChange={(event) => onChange(field.label, { enabled: event.target.checked })}
                />
              )}
            </div>
            <Textarea
              id={fieldId}
              ref={field.label === first?.label ? firstFieldRef : undefined}
              value={field.text}
              // Off is not empty: the text stays readable, which is what makes the switch a
              // decision the author can take back.
              disabled={disabled || (!field.enabled && !field.required)}
              required={field.required}
              aria-invalid={showError ? true : undefined}
              aria-describedby={describedBy || undefined}
              data-required-answer-missing={needsAnswer ? 'true' : undefined}
              rows={2}
              autoGrow
              enterKeyHint="enter"
              onChange={(event) => onChange(field.label, { text: event.target.value })}
              className="mt-1"
            />
            {field.prompt && (
              <Typography variant="meta" as="p" id={`${fieldId}-help`} className="mt-1">
                {field.prompt}
              </Typography>
            )}
            {field.required && !field.enabled && (
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <Typography variant="meta" as="p">
                  {t('editor.answers.requiredExcluded')}
                </Typography>
                {!isBlank(field.text) && (
                  <Button
                    type="button"
                    variant="secondary"
                    size="compact"
                    disabled={disabled}
                    onClick={() => onChange(field.label, { enabled: true })}
                  >
                    {t('editor.answers.restoreRequired')}
                  </Button>
                )}
              </div>
            )}
            {showError && (
              <FieldMessage id={`${fieldId}-error`} className="mt-1">
                {t(
                  field.enabled
                    ? 'editor.answers.requiredMissing'
                    : 'editor.answers.requiredExcludedError',
                )}
              </FieldMessage>
            )}
            <FieldCount left={remainingChars(field.text, TEMPLATE_LIMITS.askValue)} />
            {!field.enabled && !field.required && (
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
