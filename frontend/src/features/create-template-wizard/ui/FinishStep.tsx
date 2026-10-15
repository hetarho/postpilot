import { useId, type Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import {
  TEMPLATE_DESCRIPTION_MAX_CHARS,
  TEMPLATE_NAME_MAX_CHARS,
  TemplatePreview,
} from '@/entities/template'
import { FieldLabel, FieldMessage, Textarea, TextField } from '@/shared/ui'
import { toWizardBody } from '../lib/to-body'
import { finishProblem, type WizardEvent, type WizardState } from '../model/wizard-machine'

/** The name, an optional description and the template as it will be built (TMPL-65), before
 *  저장. A refusal from the save — a name already taken — is said under the name. */
export function FinishStep({
  state,
  send,
  refusal,
}: {
  state: WizardState
  send: Dispatch<WizardEvent>
  refusal: string
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  const problem = state.attempted ? finishProblem(state) : undefined
  const nameProblem =
    problem === 'name_missing' || problem === 'name_too_long'
      ? t(`wizard.problems.${problem}`, { max: TEMPLATE_NAME_MAX_CHARS })
      : refusal
  const descriptionProblem =
    problem === 'description_too_long'
      ? t('wizard.problems.description_too_long', { max: TEMPLATE_DESCRIPTION_MAX_CHARS })
      : ''
  return (
    <div className="space-y-6">
      <div>
        <FieldLabel htmlFor={`${id}-name`}>{t('wizard.finish.name')}</FieldLabel>
        <TextField
          id={`${id}-name`}
          value={state.name}
          aria-invalid={nameProblem ? true : undefined}
          onChange={(event) => send({ type: 'name', value: event.target.value })}
          className="mt-1"
        />
        {nameProblem && <FieldMessage className="mt-1">{nameProblem}</FieldMessage>}
      </div>
      <div>
        <FieldLabel htmlFor={`${id}-description`}>{t('wizard.finish.description')}</FieldLabel>
        <Textarea
          id={`${id}-description`}
          value={state.description}
          rows={2}
          autoGrow
          aria-invalid={descriptionProblem ? true : undefined}
          onChange={(event) => send({ type: 'description', value: event.target.value })}
          className="mt-1"
        />
        {descriptionProblem && <FieldMessage className="mt-1">{descriptionProblem}</FieldMessage>}
      </div>
      <TemplatePreview titleArea="" body={toWizardBody(state.sections)} />
    </div>
  )
}
