import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { Columns2, ScanEye, Sparkles, type LucideIcon } from 'lucide-react'
import { useRecommendationSets } from '@/entities/model-catalog'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { OptionalTestPair, StageModelRow } from '@/features/configure-model-pair'
import { PostCreditEstimate } from '@/features/select-model'
import {
  Button,
  ListRowIcon,
  LoadingState,
  Notice,
  Sheet,
  Typography,
  buttonStyles,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** Guide: AI 고르기 — model choice made friendly. It opens on the AI in use for each stage as
 *  plain rows (글 쓰기 · 사진 보기 · 말투 분석) whose 바꾸기 opens a picker, with 추천으로 적용
 *  beside the heading opening the operator's 추천 조합 in one sheet; then the two comparisons as
 *  two equal cards — writing and photo reading are the same kind of test — with 지난 비교 beside
 *  their heading; then, apart from them, the default candidates a two-model test starts from. */
export function AIModelsPage() {
  const { t } = useTranslation('models')
  const [recommending, setRecommending] = useState(false)
  return (
    // As a tab, it opens the way 글 and 영상 do: the large title and one line saying what it holds,
    // in the same column as theirs, so switching tabs moves nothing; the rows below keep the
    // reading measure.
    <main className={pageStyles({ width: 'wide', className: 'pt-2 sm:pt-6' })}>
      <Typography variant="largeTitle">{t('page.guideTitle')}</Typography>
      <Typography variant="body" className="text-content-secondary mt-1">
        {t('page.guideLead')}
      </Typography>

      <section aria-labelledby="ai-in-use" className="mt-6 lg:max-w-3xl">
        <div className="mb-2 flex min-h-11 items-center justify-between gap-3 pl-4">
          <Typography variant="label" as="h2" id="ai-in-use" className="text-content-tertiary">
            {t('page.inUse')}
          </Typography>
          <Button variant="secondary" size="compact" onClick={() => setRecommending(true)}>
            <Sparkles aria-hidden className="size-4" />
            {t('page.applyRecommendation')}
          </Button>
        </div>
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
      <RecommendationSheet open={recommending} onClose={() => setRecommending(false)} />

      <section aria-labelledby="compare-heading" className="mt-10 lg:max-w-3xl">
        <div className="mb-3 flex min-h-11 items-center justify-between gap-3">
          <Typography variant="title" as="h2" id="compare-heading">
            {t('page.compareTitle')}
          </Typography>
          <Link
            to="/tests/history"
            search={{ entry: '/ai-models' }}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-content-secondary hover:text-content-primary inline-flex min-h-11 items-center',
            })}
          >
            {t('page.testHistory')}
          </Link>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <CompareCard
            icon={Columns2}
            title={t('page.compareWrite')}
            help={t('page.compareWriteHelp')}
            stage="write"
          />
          <CompareCard
            icon={ScanEye}
            title={t('page.compareObserve')}
            help={t('page.compareObserveHelp')}
            stage="observe"
          />
        </div>
        <Typography variant="meta" as="p" className="text-content-tertiary mt-3 px-1">
          {t('page.compareNoRun')}
        </Typography>
      </section>

      {/* Which two AIs a new two-model test starts from is a setting of its own, not part of
          either comparison's invitation. */}
      <section className={cardStyles({ className: 'mt-10 shadow-sm lg:max-w-3xl' })}>
        <OptionalTestPair />
      </section>
    </main>
  )
}

/** One comparison: what it compares, in a line, and the way in. Both comparisons wear this same
 *  card, so neither reads as the main one and the other as an afterthought. */
function CompareCard({
  icon,
  title,
  help,
  stage,
}: {
  icon: LucideIcon
  title: string
  help: string
  stage: 'write' | 'observe'
}) {
  const { t } = useTranslation('models')
  const id = useId()
  return (
    <section
      aria-labelledby={id}
      className={cardStyles({ className: 'flex flex-col gap-4 shadow-sm' })}
    >
      <span className="flex items-center gap-3">
        <ListRowIcon icon={icon} />
        <Typography variant="fieldTitle" as="h3" id={id}>
          {title}
        </Typography>
      </span>
      <Typography variant="body" className="text-content-secondary flex-1">
        {help}
      </Typography>
      {stage === 'observe' ? (
        <Link
          to="/tests/photos"
          search={{ count: 2, entry: '/ai-models' }}
          className={buttonStyles({ variant: 'secondary', className: 'w-full' })}
        >
          {t('page.compareStart')}
        </Link>
      ) : (
        <Link
          to="/tests"
          search={{ factor: 'model', stage, count: 2, entry: '/ai-models' }}
          className={buttonStyles({ variant: 'secondary', className: 'w-full' })}
        >
          {t('page.compareStart')}
        </Link>
      )}
    </section>
  )
}

/** The operator's 추천 조합 (MODEL-71), every set in their order, in one sheet opened from beside
 *  지금 쓰는 AI — where the choice it changes is. Loading, failure and an installation with no set
 *  are three different answers, so none of them borrows another's copy. */
function RecommendationSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation('models')
  const id = useId()
  const recommendations = useRecommendationSets()
  return (
    <Sheet
      open={open}
      labelledBy={id}
      onClose={onClose}
      size="wide"
      header={
        <Typography variant="title" as="h2" id={id}>
          {t('page.recommendation')}
        </Typography>
      }
    >
      <Typography variant="body" className="text-content-secondary">
        {t('page.recommendationHelp')}
      </Typography>
      <div className="mt-4">
        {recommendations.isError ? (
          <Notice tone="danger" role="alert">
            {t('page.recommendationFailed')}
          </Notice>
        ) : recommendations.isPending ? (
          <LoadingState
            variant="cards"
            label={t('page.recommendationLoading')}
            className="sm:[&_.loading-reveal]:grid-cols-1 lg:[&_.loading-reveal]:grid-cols-1"
          />
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
    </Sheet>
  )
}
