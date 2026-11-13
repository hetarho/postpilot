import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ClipQuoteApproval,
  type ClipAnalysisPreparationRoute,
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
  preparationRoute?: ClipAnalysisPreparationRoute
  describedBy?: string
  onApprove(mode: ClipQuoteMode, quote: ClipQuote): void
}

/** ②'s two actions (CLIP-177, CLIP-40): 스토리라인 먼저 and 바로 만들기, a wrapping row with the careful
 *  path left and the committing one right; enlarged text can give each a full row. Each opens its own
 *  quote and approval from itself (a popover, a sheet on a phone), so the ceiling the owner
 *  approves is the one for the work that button starts. The quote is read only while it is open. */
export function ClipGenerationActions({
  stepBar = false,
  ...context
}: ClipGenerationContext & { stepBar?: boolean }) {
  const { t } = useTranslation('clips')
  const actions = (
    <>
      <ClipQuotedPopover
        {...context}
        mode="storyline"
        className={stepBar ? 'min-w-0' : 'min-w-0 grow-[3] basis-28 sm:grow-0 sm:basis-auto'}
        label={t('generation.storylineFirst')}
        variant="secondary"
      />
      <ClipQuotedPopover
        {...context}
        mode="generate"
        hero={stepBar}
        // In the step bar the committing action keeps its label on one line while a row can
        // hold it; a row too narrow for all three moves it to a full row of its own instead.
        className={
          stepBar ? 'min-w-fit flex-1' : 'min-w-0 grow-[7] basis-28 sm:grow-0 sm:basis-auto'
        }
        label={t('generation.makeNow')}
        variant="cta"
      />
    </>
  )
  return stepBar ? (
    actions
  ) : (
    <div className="flex w-full flex-wrap gap-3 sm:w-auto sm:items-center sm:justify-end">
      {actions}
    </div>
  )
}

/** ③'s storyline actions (CLIP-180, CLIP-181): 다시 만들기 writes the storyline again, and the
 *  build writes the flow and the narration from it — 이 스토리로 만들기 before there is a plan, 이
 *  스토리로 다시 만들기 once there is one. In the step bar 다시 만들기 is the row's secondary action
 *  and the build its big one (THEME-71). Each opens its own approval, and where the approved work
 *  would replace something the owner edited by hand, one confirmation says what goes; nothing asks
 *  otherwise. */
export function ClipStorylineBuildActions({
  stepBar = false,
  ...context
}: ClipGenerationContext & { hasPlan: boolean; disabled: boolean; stepBar?: boolean }) {
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
  const actions = (
    <>
      <ClipQuotedPopover
        {...shared}
        mode="storyline"
        className={stepBar ? 'min-w-0' : undefined}
        label={t('storylineActions.remake')}
        variant="secondary"
      />
      <ClipQuotedPopover
        {...shared}
        mode="fromStoryline"
        hero={stepBar}
        className={stepBar ? 'min-w-fit flex-1' : undefined}
        label={t(hasPlan ? 'storylineActions.rebuild' : 'storylineActions.build')}
        variant="cta"
      />
    </>
  )
  const confirmation = (
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
  )
  return stepBar ? (
    <>
      {actions}
      {confirmation}
    </>
  ) : (
    <div className="flex flex-wrap items-center justify-end gap-2">
      {actions}
      {confirmation}
    </div>
  )
}

/** One generation action: its trigger, and the quote and approval it opens from itself. */
export function ClipQuotedPopover({
  className,
  hero = false,
  mode,
  label,
  variant,
  ...context
}: ClipGenerationContext & {
  mode: ClipQuoteMode
  label: string
  variant: 'secondary' | 'cta'
  className?: string
  hero?: boolean
}) {
  const {
    ownerId,
    project,
    batch,
    observe,
    write,
    observeStatus,
    ready,
    pending,
    onApprove,
    preparationRoute,
    describedBy,
  } = context
  const live = ready && !!batch && !!observe && !!write
  return (
    <Popover
      className={className}
      label={label}
      triggerLabel={label}
      triggerVariant={variant}
      triggerPending={pending}
      triggerClassName={hero ? 'w-full' : 'w-full sm:w-auto'}
      triggerSize={hero ? 'hero' : undefined}
      triggerAttributes={{ 'aria-describedby': !live ? describedBy : undefined }}
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
            preparationRoute={preparationRoute}
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

export function QuotedAction({
  ownerId,
  project,
  batch,
  observe,
  write,
  observeStatus,
  mode,
  label,
  onApprove,
  preparationRoute = 'browser',
  reanalyze,
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
  preparationRoute?: ClipAnalysisPreparationRoute
  /** 다시 관찰하기's chosen sources (CLIP-234). */
  reanalyze?: readonly string[]
}) {
  const { t } = useTranslation('clips')
  const query = useClipQuote(
    ownerId,
    project,
    batch,
    observe,
    write,
    observeStatus,
    mode,
    reanalyze,
  )
  const { myPlan } = useMyPlan()
  const quote = query.quote
  const approval = quote?.recovery?.renderOnly
    ? t('credits.resumeRender')
    : quote
      ? t(
          mode === 'storyline'
            ? 'credits.approveStoryline'
            : mode === 'analysis'
              ? 'credits.approveAnalysis'
              : 'credits.approve',
          { amount: quote.maxCredits },
        )
      : label
  return (
    <ClipQuoteApproval
      quote={quote}
      quoting={query.isFetching}
      expired={query.expired}
      balance={myPlan?.balance}
      error={query.error ? appFailureFromConnect(query.error) : undefined}
      approveDisabled={!quote?.recovery?.renderOnly && !myPlan}
      approveLabel={
        preparationRoute === 'server' && !quote?.recovery?.renderOnly
          ? t('generation.serverPreparationApproval', { approval })
          : approval
      }
      onRefresh={() => void query.refetch()}
      onApprove={onApprove}
    >
      {preparationRoute === 'server' && !quote?.recovery?.renderOnly && (
        <Typography variant="body">{t('generation.serverPreparation')}</Typography>
      )}
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
