import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useRecommendationSets } from '@/entities/model-catalog'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { ActiveModelForm, OptionalTestPair } from '@/features/configure-model-pair'
import { PostCreditEstimate } from '@/features/select-model'
import { Notice, Typography, buttonStyles, pageStyles } from '@/shared/ui'
import { ModelPageHeader } from './ModelPageHeader'

export function AIModelsPage() {
  const { t } = useTranslation('models')
  const recommendations = useRecommendationSets()
  return (
    <main className={pageStyles({ className: 'pt-0 sm:pt-0 lg:pt-8' })}>
      <ModelPageHeader title="modelSettings" description="settingsDescription" />
      <div className="mt-6 space-y-6">
        <ActiveModelForm stage="observe" />
        <ActiveModelForm stage="analyze" />
        <ActiveModelForm stage="write" />
      </div>
      <PostCreditEstimate className="mt-6" />
      <section className="mt-6 sm:mt-10" aria-labelledby="writing-test-heading">
        <Typography variant="title" as="h2" id="writing-test-heading">
          {t('page.writingTests')}
        </Typography>
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('page.writingTestsHelp')}
        </Typography>
        <div className="mt-6 flex flex-wrap gap-4">
          <Link
            to="/tests"
            search={{ factor: 'model', stage: 'write', count: 2, entry: '/ai-models' }}
            className={buttonStyles({ variant: 'secondary' })}
          >
            {t('page.testWriteModels')}
          </Link>
          <Link
            to="/tests"
            search={{ factor: 'model', stage: 'observe', count: 2, entry: '/ai-models' }}
            className={buttonStyles({ variant: 'ghost' })}
          >
            {t('page.testObserveModels')}
          </Link>
          <Link
            to="/tests/history"
            search={{ entry: '/ai-models' }}
            className={buttonStyles({ variant: 'ghost' })}
          >
            {t('page.testHistory')}
          </Link>
        </div>
        <div className="mt-6">
          <OptionalTestPair />
        </div>
      </section>
      <section className="mt-6 sm:mt-10" aria-labelledby="recommendation-heading">
        <Typography variant="title" id="recommendation-heading">
          {t('page.recommendation')}
        </Typography>
        {/* Every set the operator curated, in their order (MODEL-71). Loading, failure and an
            installation with no set are three different answers, so none of them borrows
            another's copy. */}
        <div className="mt-4">
          {recommendations.isError ? (
            <Notice tone="danger" role="alert">
              {t('page.recommendationFailed')}
            </Notice>
          ) : recommendations.isPending ? (
            <Typography variant="body" role="status" className="text-content-tertiary">
              {t('page.recommendationLoading')}
            </Typography>
          ) : recommendations.sets.length === 0 ? (
            <Typography variant="body" className="text-content-tertiary">
              {t('page.recommendationEmpty')}
            </Typography>
          ) : (
            <ul className="grid gap-8">
              {recommendations.sets.map((set) => (
                <li key={set.id}>
                  <ApplyRecommendation recommendation={set} />
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
    </main>
  )
}
