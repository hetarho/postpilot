import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useStageSelection } from '@/entities/model-catalog'
import { HEADING_SUGGESTION_SECTIONS_MAX, useHeadingSuggestionsEstimate } from '@/entities/template'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import type { HeadingSuggestionsOperation } from '../model/useHeadingSuggestions'

export function HeadingSuggestions({
  operation,
  count,
  onOwnWords,
}: {
  operation: HeadingSuggestionsOperation
  count: number
  onOwnWords: () => void
}) {
  const { t } = useTranslation('templates')
  const write = useStageSelection('write')
  const estimate = useHeadingSuggestionsEstimate(write.selected)
  const tooMany = count > HEADING_SUGGESTION_SECTIONS_MAX
  return (
    <div className="space-y-3">
      <Typography variant="meta">{t('wizard.headings.styleHelp')}</Typography>
      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="secondary"
          disabled={!write.selected || operation.pending || tooMany}
          onClick={() => write.selected && operation.start(write.selected)}
        >
          {t('wizard.headings.suggest')}
        </Button>
        <Button variant="cta" onClick={onOwnWords}>
          {t('wizard.headings.ownWords')}
        </Button>
        {estimate.free ? (
          <Typography variant="meta">{t('wizard.headings.free')}</Typography>
        ) : estimate.credits !== undefined ? (
          <Typography variant="meta">
            {t('wizard.headings.credits', { count: estimate.credits })}
          </Typography>
        ) : null}
      </div>
      {!write.isPending && !write.selected && (
        <Typography variant="body">
          {t('wizard.headings.noModel')}{' '}
          <Link to="/ai-models" className="underline">
            {t('wizard.headings.chooseModel')}
          </Link>
        </Typography>
      )}
      {tooMany && (
        <Typography variant="meta">
          {t('wizard.headings.tooMany', { max: HEADING_SUGGESTION_SECTIONS_MAX })}
        </Typography>
      )}
      {operation.pending && (
        <Typography variant="body" role="status">
          {t('wizard.headings.running')}
        </Typography>
      )}
      {operation.failure && <AppFailureMessage failure={operation.failure} />}
      {operation.readFailure && (
        <div className="space-y-2">
          <AppFailureMessage failure={operation.readFailure} />
          <Button variant="secondary" onClick={operation.retryRead}>
            {t('wizard.headings.retryRead')}
          </Button>
        </div>
      )}
    </div>
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
  const { t } = useTranslation('templates')
  if (suggestions.length === 0) return null
  return (
    <div
      role="group"
      aria-label={t('wizard.headings.choices', { name })}
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
