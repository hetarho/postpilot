import { Link, useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { typographyStyles } from '@/shared/ui'
import { ExperimentReview } from './ExperimentReview'

export function ModelExperimentPage() {
  const { t } = useTranslation('models')
  const { id } = useParams({ from: '/authenticated/models/ai-models/experiments/$id' })
  const { from, stage } = useSearch({ from: '/authenticated/models/ai-models/experiments/$id' })
  return (
    <ExperimentReview
      id={id}
      backLink={(experiment) => (
        <Link
          to={from === 'compare' ? '/ai-models/compare' : '/ai-models/experiments'}
          search={{ stage: stage ?? experiment?.stage ?? 'observe' }}
          className={typographyStyles({
            variant: 'label',
            className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center',
          })}
        >
          {t(from === 'compare' ? 'experiment.backComparison' : 'experiment.backModels')}
        </Link>
      )}
    />
  )
}
