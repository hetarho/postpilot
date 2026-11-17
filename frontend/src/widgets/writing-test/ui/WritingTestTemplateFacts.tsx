import type { ReactNode } from 'react'
import { useWritingTestTranslation } from '@/features/writing-test'
import { factAnswered, factFieldId, type FactAnswer, type FactGroup } from '../lib/template-facts'
import {
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  SM_MEDIA_QUERY,
  Textarea,
  Typography,
  useMediaQuery,
} from '@/shared/ui'

/** A field's label with its marks as text, never colour alone (THEME-18): 필수 for what must be
 *  answered before a cost check, 공통 for a question several templates share. */
export function WritingFieldLabel({
  id,
  htmlFor,
  required,
  shared,
  children,
}: {
  id?: string
  htmlFor: string
  required?: boolean
  shared?: boolean
  children: ReactNode
}) {
  const { t } = useWritingTestTranslation()
  return (
    <FieldLabel id={id} htmlFor={htmlFor}>
      {children}
      {required && (
        <>
          {' '}
          <Typography variant="meta" as="span">
            {t('requiredMark')}
          </Typography>
        </>
      )}
      {shared && (
        <>
          {' '}
          <Typography variant="meta" as="span">
            {t('sharedFact')}
          </Typography>
        </>
      )}
    </FieldLabel>
  )
}

/** Each contestant template's questions under that template (MODEL-99). A phone shows one
 *  template's questions at a time behind a tab row, and from `sm:` the groups stand stacked under
 *  their headings. Only one of the two layouts is mounted, so no field exists twice. */
export function WritingTestTemplateFacts({
  groups,
  answers,
  messages,
  activeKey,
  onActiveKey,
  onAnswer,
}: {
  groups: readonly FactGroup[]
  answers: readonly FactAnswer[]
  /** The refusal under each field, by field id, once a press was refused. */
  messages: Readonly<Record<string, string>>
  activeKey: string
  onActiveKey: (key: string) => void
  onAnswer: (label: string, text: string) => void
}) {
  const { t } = useWritingTestTranslation()
  const wide = useMediaQuery(SM_MEDIA_QUERY)
  if (!groups.length) return null
  const tabbed = !wide && groups.length > 1
  const active = groups.find((group) => group.key === activeKey) ?? groups[0]
  const panelId = 'test-fact-panel'
  const asksOf = (group: FactGroup, groupIndex: number) =>
    group.asks.map((ask) => {
      const id = factFieldId(groupIndex, ask.label)
      const message = messages[id]
      return (
        <div key={ask.label} className="space-y-2">
          <WritingFieldLabel htmlFor={id} required={ask.required} shared={ask.shared}>
            {ask.label}
          </WritingFieldLabel>
          <Textarea
            id={id}
            rows={2}
            autoGrow
            value={answers.find((answer) => answer.label === ask.label)?.text ?? ''}
            aria-invalid={message ? true : undefined}
            aria-describedby={message ? `${id}-message` : undefined}
            onChange={(event) => onAnswer(ask.label, event.target.value)}
          />
          {message && <FieldMessage id={`${id}-message`}>{message}</FieldMessage>}
          {ask.prompt && <Typography variant="body">{ask.prompt}</Typography>}
        </div>
      )
    })
  return (
    <section className="space-y-4">
      <Typography variant="title">{t('templateFacts')}</Typography>
      {tabbed ? (
        <>
          <SegmentedControl
            ariaLabel={t('templateFacts')}
            controls={panelId}
            value={active.key}
            onChange={onActiveKey}
            options={groups.map((group) => ({
              value: group.key,
              label: t('factTab', {
                name: group.name,
                count: group.asks.filter((ask) => ask.required && !factAnswered(answers, ask.label))
                  .length,
              }),
            }))}
          />
          <div role="tabpanel" id={panelId} aria-label={active.name} className="space-y-4">
            {asksOf(active, groups.indexOf(active))}
          </div>
        </>
      ) : (
        groups.map((group, groupIndex) => (
          <section
            key={group.key}
            aria-labelledby={`test-fact-group-${groupIndex}`}
            className="space-y-4"
          >
            <Typography
              variant="fieldTitle"
              as="h3"
              id={`test-fact-group-${groupIndex}`}
              className="break-words"
            >
              {group.name}
            </Typography>
            {asksOf(group, groupIndex)}
          </section>
        ))
      )}
    </section>
  )
}
