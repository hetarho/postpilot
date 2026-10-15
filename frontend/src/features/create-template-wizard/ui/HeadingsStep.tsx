import { useId, type Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import { TEMPLATE_ASK_LABEL_MAX_CHARS } from '@/entities/template'
import { FieldLabel, FieldMessage, TextField } from '@/shared/ui'
import { checkedSections, type WizardEvent, type WizardState } from '../model/wizard-machine'
import type { HeadingSuggestionsOperation } from '../model/useHeadingSuggestions'
import { HeadingChoices, HeadingSuggestions } from './HeadingSuggestions'

/** A heading per section in the owner's own words; each starts at the section's name, so 다음
 *  works at once (TMPL-71). */
export function HeadingsStep({
  state,
  send,
  operation,
}: {
  state: WizardState
  send: Dispatch<WizardEvent>
  operation: HeadingSuggestionsOperation
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  return (
    <div className="space-y-6">
      <HeadingSuggestions
        operation={operation}
        count={checkedSections(state).length}
        onOwnWords={() => send({ type: 'next' })}
      />
      <ul className="space-y-4">
        {checkedSections(state).map((section) => {
          const heading = section.heading.trim()
          const problem =
            heading === ''
              ? t('wizard.problems.heading_missing')
              : Array.from(heading).length > TEMPLATE_ASK_LABEL_MAX_CHARS
                ? t('wizard.problems.heading_too_long', { max: TEMPLATE_ASK_LABEL_MAX_CHARS })
                : ''
          return (
            <li key={section.id}>
              <FieldLabel htmlFor={`${id}-${section.id}`}>
                {t('wizard.headings.field', { name: section.name })}
              </FieldLabel>
              <TextField
                id={`${id}-${section.id}`}
                value={section.heading}
                aria-invalid={state.attempted && problem ? true : undefined}
                onChange={(event) =>
                  send({ type: 'edit', id: section.id, patch: { heading: event.target.value } })
                }
                className="mt-1"
              />
              {state.attempted && problem && (
                <FieldMessage className="mt-1">{problem}</FieldMessage>
              )}
              <HeadingChoices
                name={section.name}
                suggestions={operation.suggestions(section.id)}
                onChoose={(heading) => send({ type: 'edit', id: section.id, patch: { heading } })}
              />
            </li>
          )
        })}
      </ul>
    </div>
  )
}
