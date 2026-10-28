import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { Sparkles } from 'lucide-react'
import { useStageSelection } from '@/entities/model-catalog'
import {
  HEADING_STYLE_REQUEST_MAX_CHARS,
  HEADING_SUGGESTION_SECTIONS_MAX,
  useHeadingSuggestionsEstimate,
} from '@/entities/template'
import {
  AppFailureMessage,
  Button,
  FieldCount,
  FieldLabel,
  Sheet,
  Textarea,
  Typography,
} from '@/shared/ui'
import type { WizardHeadingSuggestionsOperation as HeadingSuggestionsOperation } from '@/entities/template'

/** 제목 제안 받기 (TMPL-73): one button on the headings step that opens a sheet asking how the
 *  owner wants the headings worded. Nothing is spent before the sheet's own 제안 받기; the step
 *  keeps the progress and the result, so the owner goes on in their own words meanwhile. */
export function HeadingSuggestions({
  operation,
  count,
}: {
  operation: HeadingSuggestionsOperation
  count: number
}) {
  const { t } = useTranslation('templateTargets')
  const write = useStageSelection('write')
  const [open, setOpen] = useState(false)
  const tooMany = count > HEADING_SUGGESTION_SECTIONS_MAX
  return (
    <div className="space-y-3">
      <Button
        variant="secondary"
        disabled={!write.selected || operation.pending || tooMany}
        onClick={() => setOpen(true)}
      >
        <Sparkles aria-hidden className="size-4" />
        {t('headings.suggest')}
      </Button>
      {!write.isPending && !write.selected && (
        <Typography variant="body">
          {t('headings.noModel')}{' '}
          <Link to="/ai-models" className="underline">
            {t('headings.chooseModel')}
          </Link>
        </Typography>
      )}
      {tooMany && (
        <Typography variant="meta">
          {t('headings.tooMany', { max: HEADING_SUGGESTION_SECTIONS_MAX })}
        </Typography>
      )}
      {operation.pending && (
        <Typography variant="body" role="status">
          {t('headings.running')}
        </Typography>
      )}
      {/* While the sheet is up it says the refusal itself, beside the request it refused. */}
      {!open && operation.failure && <AppFailureMessage failure={operation.failure} />}
      {operation.readFailure && (
        <div className="space-y-2">
          <AppFailureMessage failure={operation.readFailure} />
          <Button variant="secondary" onClick={operation.retryRead}>
            {t('headings.retryRead')}
          </Button>
        </div>
      )}
      {write.selected && (
        <StyleRequestSheet
          open={open}
          operation={operation}
          writeModel={write.selected}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  )
}

/** The sheet's request stays while it is closed, so reopening it after a refusal or for another
 *  round starts from what the owner last asked. */
function StyleRequestSheet({
  open,
  operation,
  writeModel,
  onClose,
}: {
  open: boolean
  operation: HeadingSuggestionsOperation
  writeModel: Parameters<HeadingSuggestionsOperation['start']>[0]
  onClose: () => void
}) {
  const { t } = useTranslation(['templates', 'templateTargets'])
  const id = useId()
  const [request, setRequest] = useState('')
  const [sending, setSending] = useState(false)
  const estimate = useHeadingSuggestionsEstimate(writeModel)
  const chars = Array.from(request.trim()).length
  const over = chars > HEADING_STYLE_REQUEST_MAX_CHARS
  const examples = t('wizard.headings.sheet.examples', {
    ns: 'templates',
    returnObjects: true,
  }) as readonly string[]

  const send = async () => {
    if (over || sending) return
    setSending(true)
    const admitted = await operation.start(writeModel, request)
    setSending(false)
    if (admitted) onClose()
  }

  return (
    <Sheet
      open={open}
      labelledBy={`${id}-title`}
      header={
        <Typography variant="title" id={`${id}-title`}>
          {t('headings.suggest', { ns: 'templateTargets' })}
        </Typography>
      }
      // Closable while the request is being admitted: it goes on, and the step shows it.
      onClose={onClose}
    >
      <div className="space-y-5">
        <div>
          <FieldLabel htmlFor={`${id}-request`}>
            {t('wizard.headings.sheet.label', { ns: 'templates' })}
          </FieldLabel>
          <Textarea
            id={`${id}-request`}
            value={request}
            rows={3}
            autoGrow
            placeholder={t('wizard.headings.sheet.placeholder', { ns: 'templates' })}
            aria-invalid={over ? true : undefined}
            aria-describedby={`${id}-count`}
            onChange={(event) => setRequest(event.target.value)}
            className="mt-1"
          />
          <span id={`${id}-count`}>
            <FieldCount left={HEADING_STYLE_REQUEST_MAX_CHARS - chars} className="mt-1" />
          </span>
        </div>
        <div role="group" aria-labelledby={`${id}-examples`} className="space-y-2">
          <Typography variant="label" as="p" id={`${id}-examples`}>
            {t('wizard.headings.sheet.examplesLabel', { ns: 'templates' })}
          </Typography>
          <div className="flex flex-wrap gap-2">
            {examples.map((example) => (
              <Button
                key={example}
                variant="secondary"
                aria-pressed={request === example}
                onClick={() => setRequest(example)}
                className="h-auto min-h-11 py-2 text-left whitespace-normal"
              >
                {example}
              </Button>
            ))}
          </div>
        </div>
        {operation.failure && !sending && <AppFailureMessage failure={operation.failure} />}
        {/* In flow after the field, not a pinned footer: the software keyboard does not resize
            the layout viewport a pinned footer is anchored to (VOICE-53). */}
        <div className="flex flex-wrap items-center justify-end gap-3">
          {estimate.free ? (
            <Typography variant="meta">{t('headings.free', { ns: 'templateTargets' })}</Typography>
          ) : estimate.credits !== undefined ? (
            <Typography variant="meta">
              {t('headings.credits', { ns: 'templateTargets', count: estimate.credits })}
            </Typography>
          ) : null}
          <Button variant="cta" pending={sending} disabled={over} onClick={() => void send()}>
            {t('wizard.headings.sheet.send', { ns: 'templates' })}
          </Button>
        </div>
      </div>
    </Sheet>
  )
}

export function HeadingChoices({
  name,
  suggestions,
  onChoose,
}: {
  name: string
  suggestions: readonly string[]
  onChoose: (heading: string) => void
}) {
  const { t } = useTranslation('templateTargets')
  if (suggestions.length === 0) return null
  return (
    <div
      role="group"
      aria-label={t('headings.choices', { name })}
      className="mt-2 flex flex-wrap gap-2"
    >
      {suggestions.map((heading) => (
        <Button key={heading} variant="secondary" onClick={() => onChoose(heading)}>
          {heading}
        </Button>
      ))}
    </div>
  )
}
