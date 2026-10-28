import { useId, type Dispatch } from 'react'
import { useTranslation } from 'react-i18next'
import {
  TEMPLATE_ASK_LABEL_MAX_CHARS,
  TEMPLATE_ASK_MAX_PER_BODY,
  TEMPLATE_PHOTO_ROW_MAX,
} from '@/entities/template'
import {
  Checkbox,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  Switch,
  Textarea,
  TextField,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { ContentKindCards } from './ContentKindCards'
import { HeadingChoices } from './HeadingSuggestions'
import {
  sectionProblem,
  type SectionDraft,
  type SectionProblem,
  type WizardEvent,
  type WizardState,
} from '../model/wizard-machine'

/** One section's answers (TMPL-74): its heading, what stands there, its photos and, for a
 *  repeatable role, whether it repeats per photo group. Each starts at the catalog default. */
export function SectionStep({
  state,
  section,
  send,
  suggestions,
}: {
  state: WizardState
  section: SectionDraft
  send: Dispatch<WizardEvent>
  suggestions: readonly string[]
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  const edit = (patch: Partial<SectionDraft>) => send({ type: 'edit', id: section.id, patch })
  const problem = state.attempted ? sectionProblem(section, state.sections) : undefined
  const says = (kind: SectionProblem) =>
    problem === kind
      ? t(`wizard.problems.${kind}`, {
          max:
            kind === 'too_many_questions'
              ? TEMPLATE_ASK_MAX_PER_BODY
              : TEMPLATE_ASK_LABEL_MAX_CHARS,
        })
      : ''
  const headingProblem = says('heading_missing') || says('heading_too_long')
  const questionProblem =
    says('question_missing') ||
    says('question_too_long') ||
    says('question_repeated') ||
    says('too_many_questions')
  const literalProblem = says('literal_missing')
  const photoOptions = [
    { value: '0', label: t('wizard.section.photosNone') },
    ...Array.from({ length: TEMPLATE_PHOTO_ROW_MAX }, (_, index) => ({
      value: String(index + 1),
      label: t('wizard.section.photosCount', { count: index + 1 }),
    })),
  ]
  return (
    <div className="space-y-6">
      <div>
        <FieldLabel htmlFor={`${id}-heading`}>{t('wizard.section.heading')}</FieldLabel>
        <TextField
          id={`${id}-heading`}
          value={section.heading}
          aria-invalid={headingProblem ? true : undefined}
          onChange={(event) => edit({ heading: event.target.value })}
          className="mt-1"
        />
        {headingProblem && <FieldMessage className="mt-1">{headingProblem}</FieldMessage>}
        <HeadingChoices
          name={section.name}
          suggestions={suggestions}
          onChoose={(heading) => edit({ heading })}
        />
      </div>

      <div>
        <ContentKindCards section={section} onChange={(content) => edit({ content })} />
        {section.content === 'ask' && (
          <div className="mt-3">
            <FieldLabel htmlFor={`${id}-question`}>{t('wizard.section.question')}</FieldLabel>
            <TextField
              id={`${id}-question`}
              value={section.question}
              aria-invalid={questionProblem ? true : undefined}
              onChange={(event) => edit({ question: event.target.value })}
              className="mt-1"
            />
            {questionProblem && <FieldMessage className="mt-1">{questionProblem}</FieldMessage>}
            <label
              className={typographyStyles({
                variant: 'body',
                className: 'mt-2 flex min-h-11 items-center gap-3',
              })}
            >
              <Checkbox
                checked={section.required}
                onChange={(event) => edit({ required: event.target.checked })}
              />
              {t('wizard.section.required')}
            </label>
          </div>
        )}
        {section.content === 'literal' && (
          <div className="mt-3">
            <FieldLabel htmlFor={`${id}-literal`}>{t('wizard.section.literal')}</FieldLabel>
            <Textarea
              id={`${id}-literal`}
              value={section.literal}
              rows={3}
              autoGrow
              aria-invalid={literalProblem ? true : undefined}
              onChange={(event) => edit({ literal: event.target.value })}
              className="mt-1"
            />
            {literalProblem && <FieldMessage className="mt-1">{literalProblem}</FieldMessage>}
          </div>
        )}
      </div>

      <div>
        <Typography variant="label" as="p">
          {t('wizard.section.photos')}
        </Typography>
        <SegmentedControl
          value={String(section.photos)}
          ariaLabel={t('wizard.section.photos')}
          options={photoOptions}
          onChange={(value) => edit({ photos: Number(value) })}
          className="mt-2"
        />
      </div>

      {/* A 데이터 받기 field never repeats: the grammar refuses one inside a repeat. */}
      {section.repeatable && section.photos > 0 && section.content !== 'ask' && (
        <div>
          <label
            className={typographyStyles({
              variant: 'body',
              className: 'flex min-h-11 cursor-pointer items-center gap-3',
            })}
          >
            <span className="min-w-0 flex-1">{t('wizard.section.repeat')}</span>
            <Switch
              checked={section.repeat}
              aria-describedby={`${id}-repeat-help`}
              onChange={(event) => edit({ repeat: event.target.checked })}
            />
          </label>
          <Typography variant="meta" as="p" id={`${id}-repeat-help`}>
            {t('wizard.section.repeatHelp')}
          </Typography>
        </div>
      )}
    </div>
  )
}
