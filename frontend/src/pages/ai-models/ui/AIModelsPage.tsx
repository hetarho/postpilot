import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { Sparkles } from 'lucide-react'
import { FailureNotice } from '@/entities/generation-job'
import { useRecommendationSets } from '@/entities/model-catalog'
import { useSession } from '@/entities/session'
import { useWritingTests } from '@/entities/writing-test'
import { ApplyRecommendation } from '@/features/apply-model-recommendation'
import { OptionalTestPair, StageModelRow } from '@/features/configure-model-pair'
import { PostCreditEstimate } from '@/features/select-model'
import { tournamentIllustration } from '@/shared/assets'
import {
  Button,
  ListEmptyState,
  LoadingState,
  Notice,
  Sheet,
  Typography,
  WorkRowList,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { KitScreenHeader } from '@/widgets/writing-kit'
import { NewComparisonMenu, WritingTestRow } from '@/widgets/writing-test'
import { LATEST_COMPARISONS } from '../config/limits'

/** Where a comparison opened from this tab returns (THEME-58). */
const ENTRY = '/ai-models'

/** Guide: AI 토너먼트 — models compete on the owner's own work, and the winners are the ones in
 *  use. It opens on the shared section header — its title, one line, the tournament picture and
 *  the one 새 비교 menu; then 비교, the owner's latest comparisons as the work rows 지난 비교
 *  draws, with 모두 보기 beside their heading; then the AI in use for each stage as plain rows
 *  (글 쓰기 · 사진 보기 · 말투 분석) whose 바꾸기 opens a picker, with 추천으로 적용 beside the
 *  heading; then, apart from them, the default candidates a two-model test starts from. */
export function AIModelsPage() {
  const { t } = useTranslation('models')
  const [recommending, setRecommending] = useState(false)
  return (
    // The same width and title row as 글 and 영상, so moving between the tabs never shifts the
    // page under the owner; the content keeps a reading width below it.
    <main className={pageStyles({ width: 'wide', className: 'pt-2 sm:pt-6' })}>
      {/* The tab's own large title, with a section's picture beside it (THEME-81). */}
      <KitScreenHeader
        title={t('page.guideTitle')}
        statement={t('page.guideLead')}
        illustration={tournamentIllustration}
        illustrationSize="section"
        actions={<NewComparisonMenu entry={ENTRY} />}
      />
      <div className="max-w-3xl">
        {/* Guide: the tournament first — the comparisons the owner ran, a waiting one a tap
          away — then the AIs that won, in use for each stage. */}
        <section aria-labelledby="compare-heading" className="mt-8">
          <div className="mb-3 flex min-h-11 flex-wrap items-center justify-between gap-x-3">
            <Typography variant="title" as="h2" id="compare-heading">
              {t('page.compareTitle')}
            </Typography>
            <Link
              to="/tests/history"
              search={{ entry: ENTRY }}
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-content-secondary hover:text-content-primary inline-flex min-h-11 items-center',
              })}
            >
              {t('page.seeAll')}
            </Link>
          </div>
          <LatestComparisons />
        </section>

        {/* 홈's 글 쓰는 AI lands here, below the sticky header. */}
        <section id="ai-in-use" aria-labelledby="ai-in-use-heading" className="mt-10 scroll-mt-24">
          {/* The same heading row as 비교: two sections of one page, read at one level. */}
          <div className="mb-3 flex min-h-11 flex-wrap items-center justify-between gap-x-3">
            <Typography variant="title" as="h2" id="ai-in-use-heading">
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

        {/* Which two AIs a new two-model test starts from is a setting of its own, not part of
          either comparison's invitation. */}
        <section className={cardStyles({ className: 'mt-10 shadow-sm' })}>
          <OptionalTestPair />
        </section>
      </div>
    </main>
  )
}

/** The owner's latest comparisons, newest first, each the row 지난 비교 draws and opening its
 *  test with this tab as its return — a waiting comparison is one tap away. Before the first
 *  comparison the list's picture says how one is made. A read only: nothing here starts work. */
function LatestComparisons() {
  const { t } = useTranslation('models')
  const { user } = useSession()
  const query = useWritingTests(user?.id ?? '', { pageSize: LATEST_COMPARISONS })
  if (query.isError)
    return <FailureNotice message={t('page.latestFailed')} onRetry={() => void query.refetch()} />
  if (!query.data) return <LoadingState variant="table" label={t('page.latestLoading')} />
  const tests = query.data.tests
    .toSorted(
      (left, right) =>
        Date.parse(right.createdAt || right.updatedAt) -
        Date.parse(left.createdAt || left.updatedAt),
    )
    .slice(0, LATEST_COMPARISONS)
  if (tests.length === 0)
    return (
      <ListEmptyState illustration={tournamentIllustration}>{t('page.latestEmpty')}</ListEmptyState>
    )
  const entrySearch = `?entry=${encodeURIComponent(ENTRY)}`
  return (
    <WorkRowList label={t('page.compareTitle')}>
      {tests.map((test) => (
        <WritingTestRow
          key={test.id}
          test={test}
          href={`/tests/${encodeURIComponent(test.id)}${entrySearch}`}
        />
      ))}
    </WorkRowList>
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
          <ul className="grid grid-cols-[minmax(0,1fr)] gap-4">
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
