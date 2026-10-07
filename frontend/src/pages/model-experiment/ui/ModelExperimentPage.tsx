import { useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ContextualReturn, typographyStyles, useNavigationContext } from '@/shared/ui'
import { ExperimentReview } from './ExperimentReview'

export function ModelExperimentPage() {
  const { t } = useTranslation('models')
  const { id } = useParams({ from: '/authenticated/models/ai-models/experiments/$id' })
  const { from } = useSearch({ from: '/authenticated/models/ai-models/experiments/$id' })
  const navigation = useNavigationContext()
  return (
    <ExperimentReview
      id={id}
      backLink={() =>
        navigation ? (
          <ContextualReturn />
        ) : (
          <a
            href={from === 'compare' ? '/tests' : '/tests/history'}
            className={typographyStyles({
              variant: 'label',
              className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center',
            })}
          >
            {t(from === 'compare' ? 'experiment.backComparison' : 'experiment.backModels')}
          </a>
        )
      }
    />
  )
}
