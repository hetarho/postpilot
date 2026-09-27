import { useState } from 'react'
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
import { Dialog, Popover, Typography } from '@/shared/ui'
import { useClipQuote } from '../api/useClipQuote'

/** What every generation action needs to quote and start: the ready batch and the two models,
 *  the observe model's live eligibility the quote is bound to (T112), and whether the settings
 *  are ready for an approval. */
export interface ClipGenerationContext {
  ownerId: string
  project: ClipProject
  batch?: ReadyClipBatch
  observe: ModelRef | null
  write: ModelRef | null
  observeStatus: ClipEligibilityStatus | undefined
  ready: boolean
  pending: boolean
  onApprove(mode: ClipQuoteMode, quote: ClipQuote): void
}

/** ①'s two actions (CLIP-177, CLIP-40): 스토리라인 먼저 and 바로 만들기, one row split 3 : 7 on a
 *  phone as the post dock is — the careful path left, the committing one right. Each opens its own
 *  quote and approval from itself (a popover, a sheet on a phone), so the ceiling the owner
 *  approves is the one for the work that button starts. The quote is read only while it is open. */
export function ClipGenerationActions(context: ClipGenerationContext) {
  const { t } = useTranslation('clips')
  return (
    <div className="grid w-full grid-cols-[3fr_7fr] gap-3 sm:flex sm:w-auto sm:flex-wrap sm:items-center sm:justify-end">
      <ClipQuotedPopover
        {...context}
        mode="storyline"
        label={t('generation.storylineFirst')}
        variant="secondary"
      />
      <ClipQuotedPopover
        {...context}
        mode="generate"
        label={t('generation.makeNow')}
        variant="cta"
      />
    </div>
  )
}

/** ②'s storyline space's heading actions (CLIP-180, CLIP-181): 다시 만들기 writes the storyline
 *  again, and the build writes the flow and the narration from it — 이 스토리로 만들기 before there
 *  is a plan, 이 스토리로 다시 만들기 once there is one. Each opens its own approval, and where the
 *  approved work would replace something the owner edited by hand, one confirmation says what
 *  goes; nothing asks otherwise. */
export function ClipStorylineBuildActions(
  context: ClipGenerationContext & { hasPlan: boolean; disabled: boolean },
) {
  const { t } = useTranslation('clips')
  const { project, hasPlan, disabled, onApprove } = context
  const [confirming, setConfirming] = useState<{ mode: ClipQuoteMode; quote: ClipQuote }>()
  const guarded = (mode: ClipQuoteMode, quote: ClipQuote) => {
    const replacesHandwork =
      mode === 'storyline' ? !!project.storyline?.editedByHand : !!project.planEditedByHand
    if (replacesHandwork) setConfirming({ mode, quote })
    else onApprove(mode, quote)
  }
  const shared = { ...context, ready: context.ready && !disabled, onApprove: guarded }
  return (
    <div className="flex flex-wrap items-center justify-end gap-2">
      <ClipQuotedPopover
        {...shared}
        mode="storyline"
        label={t('storylineActions.remake')}
        variant="secondary"
      />
      <ClipQuotedPopover
        {...shared}
        mode="fromStoryline"
        label={t(hasPlan ? 'storylineActions.rebuild' : 'storylineActions.build')}
        variant="cta"
      />
      <Dialog
        open={!!confirming}
        title={t(
          confirming?.mode === 'storyline'
            ? 'storylineActions.remakeTitle'
            : 'storylineActions.rebuildTitle',
        )}
        confirmLabel={t(
          confirming?.mode === 'storyline' ? 'storylineActions.remake' : 'storylineActions.rebuild',
        )}
        onClose={() => setConfirming(undefined)}
        onConfirm={() => {
          if (confirming) onApprove(confirming.mode, confirming.quote)
          setConfirming(undefined)
        }}
      >
        {t(
          confirming?.mode === 'storyline'
            ? 'storylineActions.remakeBody'
            : 'storylineActions.rebuildBody',
        )}
      </Dialog>
    </div>
  )
}

/** One generation action: its trigger, and the quote and approval it opens from itself. */
function ClipQuotedPopover({
  mode,
  label,
  variant,
  ...context
}: ClipGenerationContext & {
  mode: ClipQuoteMode
  label: string
  variant: 'secondary' | 'cta'
}) {
  const { ownerId, project, batch, observe, write, observeStatus, ready, pending, onApprove } =
    context
  const live = ready && !!batch && !!observe && !!write
  return (
    <Popover
      label={label}
      triggerLabel={label}
      triggerVariant={variant}
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
            label={label}
            onApprove={(quote) => {
              close()
              onApprove(mode, quote)
            }}
          />
        ) : null
      }
    </Popover>
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
  label,
  onApprove,
}: {
  ownerId: string
  project: ClipProject
  batch: ReadyClipBatch
  observe: ModelRef
  write: ModelRef
  observeStatus: ClipEligibilityStatus | undefined
  mode: ClipQuoteMode
  label: string
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
            : label
      }
      onRefresh={() => void query.refetch()}
      onApprove={onApprove}
    >
      {quote?.recovery ? (
        <Typography variant="body" role="status">
          {quote.recovery.renderOnly
            ? t('credits.renderOnly')
            : t('credits.reuse', {
                done: quote.recovery.reusedChunks,
                remaining: quote.recovery.remainingChunks,
                retries: quote.recovery.responseRetries,
              })}
        </Typography>
      ) : (
        // A prior attempt that left no compatible recovery data says what restarts rather than
        // implying its work carries over (CLIP-96).
        quote &&
        (project.latestJob?.status === 'failed' || project.latestJob?.status === 'cancelled') && (
          <Typography variant="body" role="status">
            {t('credits.restart')}
          </Typography>
        )
      )}
    </ClipQuoteApproval>
  )
}
