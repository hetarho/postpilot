import { useTranslation } from 'react-i18next'
import {
  ClipQuoteApproval,
  type ClipEligibilityStatus,
  type ClipProject,
  type ClipQuote,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import type { ModelRef } from '@/entities/model-catalog'
import { useMyPlan } from '@/entities/plan'
import { appFailureFromConnect } from '@/shared/api'
import { Button, Typography } from '@/shared/ui'
import { useClipQuote } from '../api/useClipQuote'

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
  return (
    <ClipQuoteApproval
      quote={quote}
      quoting={query.isFetching}
      expired={query.expired}
      balance={myPlan?.balance}
      error={query.error ? appFailureFromConnect(query.error) : undefined}
      approveDisabled={!quote?.recovery?.renderOnly && !myPlan}
      approveLabel={
        quote?.recovery?.renderOnly
          ? t('credits.resumeRender')
          : quote
            ? t('credits.approve', { amount: quote.maxCredits })
            : t('generation.generate')
      }
      onRefresh={() => void query.refetch()}
      onApprove={onApprove}
    >
      {quote?.recovery && (
        <Typography variant="body" role="status">
          {quote.recovery.renderOnly
            ? t('credits.renderOnly')
            : t('credits.reuse', {
                done: quote.recovery.reusedChunks,
                remaining: quote.recovery.remainingChunks,
                retries: quote.recovery.responseRetries,
              })}
        </Typography>
      )}
    </ClipQuoteApproval>
  )
}
