import { useTranslation } from 'react-i18next'
import { useModelSetup } from '@/entities/model-catalog'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { ActiveModelForm } from '@/features/configure-model-pair'
import { PostCreditEstimate } from '@/features/select-model'
import { Typography, pageStyles } from '@/shared/ui'
import { ModelPageHeader } from './ModelPageHeader'

export function AIModelsPage() {
  const { t } = useTranslation('models')
  const setup = useModelSetup()
  return (
    <main className={pageStyles({ className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="modelSettings" description="settingsDescription" />
      <div className="mt-6 space-y-6">
        <ActiveModelForm stage="observe" />
        <ActiveModelForm stage="analyze" />
        <ActiveModelForm stage="write" />
      </div>
      <PostCreditEstimate className="mt-6" />
      <section className="mt-10" aria-labelledby="recommendation-heading">
        <Typography variant="title" id="recommendation-heading">
          {t('page.recommendation')}
        </Typography>
        <div className="mt-4">
          {setup.recommendations[0] ? (
            <ApplyRecommendation recommendation={setup.recommendations[0]} />
          ) : (
            <Typography variant="body" className="text-content-tertiary">
              {t('page.recommendationLoading')}
            </Typography>
          )}
        </div>
      </section>
    </main>
  )
}
