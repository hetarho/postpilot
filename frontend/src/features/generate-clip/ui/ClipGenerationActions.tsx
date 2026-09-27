import { useTranslation } from 'react-i18next'
import {
  ClipQuoteApproval,
  type ClipEligibilityStatus,
  type ClipProject,
  type ClipQuote,
  type ClipQuoteMode,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import type { ModelRef } from '@/entities/model-catalog'
import { useMyPlan } from '@/entities/plan'
import { appFailureFromConnect } from '@/shared/api'
import { Popover, Typography } from '@/shared/ui'
import { useClipQuote } from '../api/useClipQuote'

/** ①'s two actions (CLIP-177, CLIP-40): 스토리라인 먼저 and 바로 만들기, one row split 3 : 7 on a
 *  phone as the post dock is — the careful path left, the committing one right. Each opens its own
 *  quote and approval from itself (a popover, a sheet on a phone), so the ceiling the owner
 *  approves is the one for the work that button starts. The quote is read only while it is open. */
export function ClipGenerationActions({
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
  onApprove(mode: Extract<ClipQuoteMode, 'storyline' | 'generate'>, quote: ClipQuote): void
}) {
  const { t } = useTranslation('clips')
  const live = ready && !!batch && !!observe && !!write
  const action = (mode: 'storyline' | 'generate') => (
    <Popover
      label={t(mode === 'storyline' ? 'generation.storylineFirst' : 'generation.makeNow')}
      triggerLabel={t(mode === 'storyline' ? 'generation.storylineFirst' : 'generation.makeNow')}
      triggerVariant={mode === 'storyline' ? 'secondary' : 'cta'}
      triggerPending={pending}
      triggerClassName="w-full sm:w-auto"
      disabled={!live || pending}
      placement="above"
      align="end"
      phone="sheet"
    >
      {(close) =>
        live && batch && observe && write ? (
          <QuotedAction
            ownerId={ownerId}
            project={project}
            batch={batch}
            observe={observe}
            write={write}
            observeStatus={observeStatus}
            mode={mode}
            onApprove={(quote) => {
              onApprove(mode, quote)
              close()
            }}
          />
        ) : null
      }
    </Popover>
  )
  return (
    <div className="grid w-full grid-cols-[3fr_7fr] gap-3 sm:flex sm:w-auto sm:flex-wrap sm:items-center sm:justify-end">
      {action('storyline')}
      {action('generate')}
    </div>
  )
}

function QuotedAction({
  ownerId,
  project,
  batch,
  observe,
  write,
  observeStatus,
  mode,
  onApprove,
}: {
  ownerId: string
  project: ClipProject
  batch: ReadyClipBatch
  observe: ModelRef
  write: ModelRef
  observeStatus: ClipEligibilityStatus | undefined
  mode: 'storyline' | 'generate'
  onApprove(quote: ClipQuote): void
}) {
  const { t } = useTranslation('clips')
  const query = useClipQuote(ownerId, project, batch, observe, write, observeStatus, mode)
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
            ? t(mode === 'storyline' ? 'credits.approveStoryline' : 'credits.approve', {
                amount: quote.maxCredits,
              })
            : t(mode === 'storyline' ? 'generation.storylineFirst' : 'generation.makeNow')
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
