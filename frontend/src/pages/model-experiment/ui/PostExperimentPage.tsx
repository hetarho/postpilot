import { Link, useParams, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { typographyStyles } from '@/shared/ui'
import { ExperimentReview } from './ExperimentReview'

export function PostExperimentPage() {
  const { t } = useTranslation('models')
  const { id } = useParams({ from: '/authenticated/writing/posts/experiments/$id' })
  const { from, q, status } = useSearch({ from: '/authenticated/writing/posts/experiments/$id' })
  const className = typographyStyles({
    variant: 'label',
    className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center',
  })
  return (
    <ExperimentReview
      id={id}
      backLink={(experiment) =>
        from !== 'posts' && experiment?.postSlug ? (
          <Link to="/posts/$slug" params={{ slug: experiment.postSlug }} className={className}>
            {t('experiment.backPost')}
          </Link>
        ) : (
          <Link to="/posts" search={{ q, status }} className={className}>
            {t('experiment.backPosts')}
          </Link>
        )
      }
    />
  )
}
