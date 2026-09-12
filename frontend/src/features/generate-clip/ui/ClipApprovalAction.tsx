import { useTranslation } from 'react-i18next'
import type {
  ClipEligibilityStatus,
  ClipProject,
  ClipQuote,
  ReadyClipBatch,
} from '@/entities/clip-project'
import type { ModelRef } from '@/entities/model-catalog'
import { useMyPlan } from '@/entities/plan'
import { appFailureFromConnect } from '@/shared/api'
import { Button, Typography } from '@/shared/ui'
import { useClipQuote } from '../api/useClipQuote'
import { ClipGenerationFailure } from './ClipGenerationFailure'

export function ClipApprovalAction({
  ownerId,
  project,
  batch,
  observe,
  write,
  observeStatus,
  ready,
  pending,
  onApprove,
}: {
  ownerId: string
  project: ClipProject
  batch?: ReadyClipBatch
  observe: ModelRef | null
  write: ModelRef | null
  /** The observe model's live eligibility the quote is bound to (T112). */
  observeStatus: ClipEligibilityStatus | undefined
  ready: boolean
  pending: boolean
  onApprove(quote: ClipQuote): void
}) {
  const { t } = useTranslation('clips')
  return ready && batch && observe && write ? (
    <QuotedAction
      ownerId={ownerId}
      project={project}
      batch={batch}
      observe={observe}
      write={write}
      observeStatus={observeStatus}
      onApprove={onApprove}
    />
  ) : (
    <Button variant="secondary" className="w-full sm:w-auto" disabled pending={pending}>
      {t(
        project.result || ['failed', 'cancelled'].includes(project.latestJob?.status ?? '')
          ? 'generation.retry'
          : 'generation.generate',
      )}
    </Button>
  )
}
function QuotedAction({
  ownerId,
  project,
  batch,
  observe,
  write,
  observeStatus,
  onApprove,
}: {
  ownerId: string
  project: ClipProject
  batch: ReadyClipBatch
  observe: ModelRef
  write: ModelRef
  observeStatus: ClipEligibilityStatus | undefined
  onApprove(quote: ClipQuote): void
}) {
  const { t } = useTranslation('clips')
  const query = useClipQuote(ownerId, project, batch, observe, write, observeStatus)
  const { myPlan } = useMyPlan()
  const quote = query.quote
  const insufficient =
    !!quote && !!myPlan && !myPlan.balance.unlimited && quote.maxCredits > myPlan.balance.credits
  return (
    <div className="w-full min-w-0 space-y-2">
      <Typography variant="meta" role="status">
        {query.isFetching
          ? t('credits.quoting')
          : query.expired
            ? t('credits.expired')
            : t('credits.maximumHelp')}
      </Typography>
      <Typography variant="body">
        {t(quote?.cancellationPolicy ? 'cancellation.rule' : 'cancellation.policyUnavailable')}
      </Typography>
      {myPlan?.balance.unlimited && <Typography variant="body">{t('credits.exempt')}</Typography>}
      {query.error && <ClipGenerationFailure failure={appFailureFromConnect(query.error)} />}
      {insufficient && myPlan && quote && (
        <ClipGenerationFailure
          failure={{
            reason: 'INSUFFICIENT_CREDITS',
            params: {
              required: String(quote.maxCredits),
              balance: String(myPlan.balance.credits),
              renews_at: myPlan.balance.renewsAt,
            },
          }}
        />
      )}
      {(query.expired || query.isError) && (
        <Button variant="secondary" pending={query.isFetching} onClick={() => void query.refetch()}>
          {t('credits.refresh')}
        </Button>
      )}
      <Button
        variant="cta"
        className="w-full whitespace-normal"
        disabled={!quote?.cancellationPolicy || !myPlan || insufficient}
        pending={query.isFetching}
        onClick={() => {
          if (quote?.cancellationPolicy && !insufficient) onApprove(quote)
        }}
      >
        {quote ? t('credits.approve', { amount: quote.maxCredits }) : t('generation.generate')}
      </Button>
    </div>
  )
}
