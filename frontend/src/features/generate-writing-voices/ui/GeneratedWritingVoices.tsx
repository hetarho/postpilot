import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import {
  useStageSelection,
  useModels,
  useSelections,
  useInitializeDefaultSelections,
} from '@/entities/model-catalog'
import { useJob, isTerminal } from '@/entities/generation-job'
import type { Voice } from '@/entities/voice'
import {
  useLatestWritingVoiceCandidates,
  useEstimateWritingVoiceCandidates,
  useStartWritingVoiceCandidates,
  useCancelWritingVoiceCandidates,
  useAdoptWritingVoiceCandidate,
} from '@/entities/voice-candidate'
import { appFailureFromConnect } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  Dialog,
  Notice,
  Sheet,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import {
  initialCandidateState,
  candidateTransition,
  candidateBusy,
  type CandidateEvent,
} from '../model/candidate-machine'

export interface GeneratedWritingVoicesProps {
  ownerId: string
  onAdopted: (voice: Voice) => void
  onBusyChange?: (busy: boolean) => void
}
export function GeneratedWritingVoices(props: GeneratedWritingVoicesProps) {
  return <AccountCandidates key={props.ownerId} {...props} />
}
function AccountCandidates({ ownerId, onAdopted, onBusyChange }: GeneratedWritingVoicesProps) {
  const { t } = useTranslation('voices')
  const [state, setState] = useState(() => initialCandidateState(ownerId))
  const current = useRef(state)
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const send = useCallback((event: CandidateEvent) => {
    const next = candidateTransition(current.current, event)
    if (mounted.current) {
      current.current = next
      setState(next)
    }
    return next
  }, [])
  const latest = useLatestWritingVoiceCandidates(ownerId)
  const preparation = useInitializeDefaultSelections(ownerId)
  const writing = useStageSelection('write')
  const models = useModels()
  const selections = useSelections()
  const start = useStartWritingVoiceCandidates(ownerId)
  const cancel = useCancelWritingVoiceCandidates(ownerId)
  const adopt = useAdoptWritingVoiceCandidate(ownerId)
  const quote = useEstimateWritingVoiceCandidates(
    ownerId,
    state.phase === 'confirming' ? state.frozenModel : undefined,
  )
  const terminalKeys = useMemo(() => [latest.queryKey], [latest.queryKey])
  const progress = useJob(state.jobId, terminalKeys)
  useEffect(() => {
    if (latest.batch) send({ type: 'latest', ownerId, batch: latest.batch })
  }, [latest.batch, ownerId, send, state.phase, state.jobId])
  useEffect(() => {
    const job = progress.job
    if (job && isTerminal(job))
      send({ type: 'terminal', ownerId, jobId: job.id, status: job.status, failure: job.failure })
  }, [progress.job, ownerId, send])
  useEffect(() => {
    if (latest.isError && progress.job?.status === 'done')
      send({ type: 'result-failed', ownerId, jobId: progress.job.id, failure: latest.failure })
  }, [latest.isError, latest.failure, progress.job, ownerId, send])
  const busy = candidateBusy(state)
  const busyCallback = useRef(onBusyChange)
  useEffect(() => {
    busyCallback.current = onBusyChange
  }, [onBusyChange])
  useEffect(() => {
    busyCallback.current?.(busy)
  }, [busy])
  useEffect(() => () => busyCallback.current?.(false), [])
  const openConfirmation = () => {
    if (
      !preparation.isPending &&
      !preparation.isError &&
      !latest.isPending &&
      !latest.isError &&
      writing.selected
    )
      send({ type: 'confirm', ownerId, model: writing.selected })
  }
  const run = async () => {
    const before = current.current
    const price = quote.estimate
    if (
      preparation.isPending ||
      preparation.isError ||
      quote.isFetching ||
      !price ||
      (!price.free && (price.credits === undefined || price.credits < 0))
    )
      return
    const next = send({ type: 'start', ownerId })
    if (next === before || !next.frozenModel) return
    try {
      const result = await start.start(next.frozenModel)
      if (mounted.current)
        send({ type: 'started', ownerId, operation: next.operation, jobId: result.jobId })
    } catch (error) {
      if (mounted.current)
        send({
          type: 'failure',
          ownerId,
          operation: next.operation,
          failure: appFailureFromConnect(error),
        })
    }
  }
  const stop = async () => {
    const before = current.current
    const next = send({ type: 'cancel', ownerId })
    if (next === before) return
    try {
      await cancel.cancel(next.jobId)
      if (mounted.current) send({ type: 'cancelled', ownerId, operation: next.operation })
      progress.refetch()
    } catch (error) {
      if (mounted.current)
        send({
          type: 'failure',
          ownerId,
          operation: next.operation,
          failure: appFailureFromConnect(error),
        })
    }
  }
  const adoptSelected = async () => {
    const before = current.current
    const next = send({ type: 'adopt', ownerId })
    if (next === before) return
    try {
      const voice = await adopt.adopt({ jobId: next.resultJobId, candidateId: next.selectedId })
      if (
        !mounted.current ||
        current.current.operation !== next.operation ||
        current.current.phase !== 'adopting'
      )
        return
      send({ type: 'adopted', ownerId, operation: next.operation })
      onAdopted(voice)
    } catch (error) {
      if (mounted.current)
        send({
          type: 'failure',
          ownerId,
          operation: next.operation,
          failure: appFailureFromConnect(error),
        })
    }
  }
  const confirmTitle = useId()
  const priceReady =
    !!quote.estimate &&
    !quote.isFetching &&
    !quote.isError &&
    (quote.estimate.free || (quote.estimate.credits !== undefined && quote.estimate.credits >= 0))
  const canGenerate =
    !!writing.selected &&
    !writing.isPending &&
    !preparation.isPending &&
    !preparation.isError &&
    !busy &&
    !latest.isPending &&
    !latest.isError
  const canChoose = ['ready', 'failed'].includes(state.phase)
  return (
    <section
      className="@container flex min-w-0 flex-col gap-6"
      aria-label={t('candidateFlow.title')}
    >
      <div className="flex flex-col gap-3">
        <Typography variant="title">{t('candidateFlow.title')}</Typography>
        <Typography variant="body" className="text-content-secondary">
          {t('candidateFlow.intro')}
        </Typography>
      </div>
      {latest.isPending && (
        <Typography variant="body" role="status">
          {t('candidateFlow.loading')}
        </Typography>
      )}
      {latest.isError && (
        <Notice tone="danger" role="alert">
          {latest.failure ? (
            <AppFailureMessage failure={latest.failure} />
          ) : (
            t('candidateFlow.loadFailed')
          )}
          <Button variant="ghost" onClick={latest.refetch}>
            {t('candidateFlow.retry')}
          </Button>
        </Notice>
      )}
      {(!writing.selected || preparation.isError) && (
        <Notice tone="info" role="status">
          {t(
            writing.isPending || preparation.isPending
              ? 'candidateFlow.preparing'
              : 'candidateFlow.unavailable',
          )}
          {!writing.isPending && !preparation.isPending && (
            <>
              <Button
                variant="ghost"
                onClick={() => {
                  models.refetch()
                  selections.refetch()
                  preparation.retry()
                }}
              >
                {t('candidateFlow.retry')}
              </Button>
              <a
                href="/ai-models"
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-link-fg underline',
                })}
              >
                {t('candidateFlow.settings')}
              </a>
            </>
          )}
        </Notice>
      )}
      {(state.phase === 'running' ||
        state.phase === 'cancelling' ||
        state.phase === 'starting') && (
        <Notice tone="info" role="status">
          <Typography variant="body">
            {t(state.phase === 'cancelling' ? 'candidateFlow.cancelling' : 'candidateFlow.running')}
          </Typography>
          {state.candidates.length > 0 && (
            <Typography variant="meta">{t('candidateFlow.retained')}</Typography>
          )}
          {state.phase === 'running' && (
            <Button variant="ghost" onClick={() => send({ type: 'open-cancel', ownerId })}>
              {t('candidateFlow.cancel')}
            </Button>
          )}
        </Notice>
      )}
      {progress.isError && busy && (
        <Notice tone="warning" role="alert">
          {t('candidateFlow.statusFailed')}
          <Button variant="ghost" onClick={progress.refetch}>
            {t('candidateFlow.retry')}
          </Button>
        </Notice>
      )}
      {(state.failure || state.phase === 'failed') && (
        <Notice tone="danger" role="alert">
          {state.failure ? (
            <AppFailureMessage failure={state.failure} />
          ) : (
            t('candidateFlow.failed')
          )}
        </Notice>
      )}
      {state.candidates.length > 0 && (
        <>
          <Typography variant="body" className="text-content-secondary">
            {t('candidateFlow.ready')}
          </Typography>
          <ul className="grid min-w-0 grid-cols-1 gap-4 @lg:grid-cols-2 @6xl:grid-cols-4">
            {state.candidates.map((candidate) => (
              <li key={`${state.resultJobId}:${candidate.id}`} className="min-w-0">
                <article
                  className={clsx(
                    'flex h-full min-w-0 flex-col gap-4 rounded-lg p-5',
                    state.selectedId === candidate.id ? 'bg-surface-recessed' : 'bg-surface-raised',
                  )}
                >
                  <Typography variant="eyebrow" className="text-content-secondary">
                    {t('writingCandidates.created')}
                  </Typography>
                  <Typography variant="fieldTitle" id={`${confirmTitle}-${candidate.id}`}>
                    {candidate.name}
                  </Typography>
                  <Typography variant="body" className="text-content-secondary">
                    {candidate.description}
                  </Typography>
                  <Typography variant="meta" className="text-content-secondary">
                    {t('writingCandidates.fictional')}
                  </Typography>
                  <Typography
                    variant="body"
                    as="blockquote"
                    className="flex-1 break-words whitespace-pre-wrap"
                  >
                    {candidate.sample}
                  </Typography>
                  <Button
                    variant="secondary"
                    aria-describedby={`${confirmTitle}-${candidate.id}`}
                    aria-pressed={state.selectedId === candidate.id}
                    disabled={!canChoose}
                    onClick={() => send({ type: 'select', ownerId, candidateId: candidate.id })}
                  >
                    {t(
                      state.selectedId === candidate.id
                        ? 'writingCandidates.selected'
                        : 'writingCandidates.choose',
                    )}
                  </Button>
                </article>
              </li>
            ))}
          </ul>
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
            <Button
              variant="cta"
              disabled={!canChoose || !state.selectedId}
              pending={state.phase === 'adopting'}
              onClick={() => void adoptSelected()}
            >
              {t('candidateFlow.adopt')}
            </Button>
            <Button variant="secondary" disabled={!canGenerate} onClick={openConfirmation}>
              {t('candidateFlow.reroll')}
            </Button>
            {!state.selectedId && (
              <Typography variant="meta" role="status">
                {t('candidateFlow.selectFirst')}
              </Typography>
            )}
          </div>
        </>
      )}
      {state.candidates.length === 0 && (
        <Button
          variant="cta"
          className="self-start"
          disabled={!canGenerate}
          onClick={openConfirmation}
        >
          {t('candidateFlow.generate')}
        </Button>
      )}
      <Sheet
        open={state.phase === 'confirming' || state.phase === 'starting'}
        labelledBy={confirmTitle}
        onClose={() => send({ type: 'dismiss-confirm', ownerId })}
        header={
          <Typography variant="title" id={confirmTitle}>
            {t('candidateFlow.confirming')}
          </Typography>
        }
        footer={
          <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
            <Button
              variant="ghost"
              disabled={state.phase === 'starting'}
              onClick={() => send({ type: 'dismiss-confirm', ownerId })}
            >
              {t('candidateFlow.close')}
            </Button>
            <Button
              variant="cta"
              disabled={!priceReady}
              pending={state.phase === 'starting'}
              onClick={() => void run()}
            >
              {t('candidateFlow.confirm')}
            </Button>
          </div>
        }
      >
        <div className="flex flex-col gap-4">
          {quote.isPending || quote.isFetching ? (
            <Typography variant="body" role="status">
              {t('candidateFlow.estimating')}
            </Typography>
          ) : quote.isError || !priceReady ? (
            <Notice tone="danger" role="alert">
              {quote.failure ? (
                <AppFailureMessage failure={quote.failure} />
              ) : (
                t('candidateFlow.estimateFailed')
              )}
              <Button variant="ghost" onClick={quote.refetch}>
                {t('candidateFlow.retry')}
              </Button>
            </Notice>
          ) : (
            <Typography variant="body">
              {quote.estimate?.free
                ? t('candidateFlow.free')
                : t('candidateFlow.quote', { credits: quote.estimate?.credits ?? 0 })}
            </Typography>
          )}
          <Typography variant="body" className="text-content-secondary">
            {t('candidateFlow.actual')}
          </Typography>
        </div>
      </Sheet>
      <Dialog
        open={state.cancelDialog}
        title={t('candidateFlow.cancelTitle')}
        confirmLabel={t('candidateFlow.cancel')}
        cancelLabel={t('candidateFlow.keepRunning')}
        pending={state.phase === 'cancelling'}
        onConfirm={() => void stop()}
        onClose={() => send({ type: 'dismiss-cancel', ownerId })}
      >
        <Typography variant="body">{t('candidateFlow.cancelExplanation')}</Typography>
      </Dialog>
    </section>
  )
}
