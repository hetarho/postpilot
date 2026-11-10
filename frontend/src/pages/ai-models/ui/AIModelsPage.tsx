import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { ChevronRight, Columns2, History, ScanEye } from 'lucide-react'
import { useRecommendationSets } from '@/entities/model-catalog'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { OptionalTestPair, StageModelRow } from '@/features/configure-model-pair'
import { PostCreditEstimate } from '@/features/select-model'
import {
  ListRowIcon,
  Notice,
  Typography,
  buttonStyles,
  cardStyles,
  listRowStyles,
  pageStyles,
} from '@/shared/ui'

/** Guide: AI 고르기 — model choice made friendly. It opens on the AI in use for each stage as
 *  plain rows (글 쓰기 · 사진 보기 · 말투 분석) whose 바꾸기 opens a picker sorted by what matters
 *  first; then the ONE big invitation to 비교해 보기 — two AIs writing the owner's own post side by
 *  side, the writing test and the model comparison as one flow — with 지난 비교 beside it; then the
 *  operator's 추천 조합. */
export function AIModelsPage() {
  const { t } = useTranslation('models')
  const recommendations = useRecommendationSets()
  return (
    <main className={pageStyles({ width: 'prose', className: 'pt-4 sm:pt-10' })}>
      <Typography variant="largeTitle">{t('page.guideTitle')}</Typography>

      <section aria-labelledby="ai-in-use" className="mt-8">
        <Typography
          variant="label"
          as="h2"
          id="ai-in-use"
          className="text-content-tertiary mb-2 px-4"
        >
          {t('page.inUse')}
        </Typography>
        <div
          className={cardStyles({
            padding: 'none',
            className: 'divide-divider divide-y overflow-hidden shadow-sm',
          })}
        >
          <StageModelRow stage="write" />
          <StageModelRow stage="observe" />
          <StageModelRow stage="analyze" />
        </div>
        <PostCreditEstimate className="mt-3 px-4" />
      </section>

      {/* The comparison: the screen's one big action. */}
      <section
        aria-labelledby="writing-test-heading"
        className={cardStyles({
          padding: 'lg',
          className: 'bg-badge-accent-bg animate-step-in mt-10 flex flex-col gap-5',
        })}
      >
        <span className="flex items-center gap-3">
          <ListRowIcon icon={Columns2} size="lg" />
          <Typography variant="title" as="h2" id="writing-test-heading" className="text-balance">
            {t('page.writingTests')}
          </Typography>
        </span>
        <Link
          to="/tests"
          search={{ factor: 'model', stage: 'write', count: 2, entry: '/ai-models' }}
          className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full' })}
        >
          {t('page.testWriteModels')}
        </Link>
        <OptionalTestPair />
      </section>

      <ul
        className={cardStyles({
          padding: 'none',
          className: 'divide-divider mt-6 divide-y overflow-hidden shadow-sm',
        })}
      >
        <li>
          <Link
            to="/tests"
            search={{ factor: 'model', stage: 'observe', count: 2, entry: '/ai-models' }}
            className={listRowStyles()}
          >
            <ListRowIcon icon={ScanEye} />
            <span className="min-w-0 flex-1">{t('page.testObserveModels')}</span>
            <ChevronRight aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
          </Link>
        </li>
        <li>
          <Link to="/tests/history" search={{ entry: '/ai-models' }} className={listRowStyles()}>
            <ListRowIcon icon={History} />
            <span className="min-w-0 flex-1">{t('page.testHistory')}</span>
            <ChevronRight aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
          </Link>
        </li>
      </ul>

      <section className="mt-10" aria-labelledby="recommendation-heading">
        <Typography variant="title" as="h2" id="recommendation-heading">
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
            <ul className="grid gap-4">
              {recommendations.sets.map((set) => (
                <li key={set.id} className={cardStyles({ className: 'shadow-sm' })}>
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
