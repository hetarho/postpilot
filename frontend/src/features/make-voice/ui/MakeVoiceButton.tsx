import { useEffect, useId, useRef } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStageSelection } from '@/entities/model-catalog'
import { useJob, isTerminal } from '@/entities/generation-job'
import {
  useAnalyzeVoice,
  useVoiceAnalysisEstimator,
  useVoiceProfile,
  voiceMaterialVersionKey,
  type VoiceProfile,
} from '@/entities/voice'
import {
  AppFailureMessage,
  Button,
  Dialog,
  FieldMessage,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { analysisConfirmMachine } from '../model/analysis-confirm-machine'

type MakeVoiceProps = {
  ownerId: string
  voiceId: string
  profile: Pick<VoiceProfile, 'made' | 'readiness' | 'activeJobId' | 'voice'> &
    Partial<Pick<VoiceProfile, 'samples'>>
  onStarted: (jobId: string) => void
}
export function MakeVoiceButton(props: MakeVoiceProps) {
  return <ScopedMakeVoice key={JSON.stringify([props.ownerId, props.voiceId])} {...props} />
}
function ScopedMakeVoice({ ownerId, voiceId, profile, onStarted }: MakeVoiceProps) {
  const { t } = useTranslation('voices')
  const reasonId = useId()
  const scopeKey = ownerId && voiceId ? JSON.stringify([ownerId, voiceId]) : ''
  const { selected, isPending: modelPending } = useStageSelection('analyze')
  const analyze = useAnalyzeVoice(ownerId, voiceId)
  const estimator = useVoiceAnalysisEstimator(ownerId, voiceId)
  const profileReader = useVoiceProfile(ownerId, voiceId)
  const runtime = useRef({
    estimate: estimator.estimate,
    analyze: async (model: { providerId: string; modelId: string }) =>
      (await analyze.analyze(model)).jobId,
    read: profileReader.refresh,
  })
  useEffect(() => {
    runtime.current = {
      estimate: estimator.estimate,
      analyze: async (model) => (await analyze.analyze(model)).jobId,
      read: profileReader.refresh,
    }
  }, [estimator, analyze, profileReader.refresh])
  const actor = useActorRef(analysisConfirmMachine, { input: { scopeKey, runtime } })
  const snapshot = useSelector(actor, (value) => value)
  const ready = profile.readiness.percent >= 100
  const noModel = !modelPending && !selected
  const reason = profile.voice.deleted
    ? t('make.deleted')
    : !ready
      ? t('make.notReady')
      : noModel
        ? t('make.noModel')
        : ''
  const available = !!scopeKey && !reason && !modelPending && !profile.activeJobId
  const versionKey = voiceMaterialVersionKey({ samples: profile.samples ?? [] })
  useEffect(() => {
    actor.send({ type: 'FACTS', scopeKey, versionKey, available, activeJobId: profile.activeJobId })
  }, [actor, scopeKey, versionKey, available, profile.activeJobId])
  const job = useJob(snapshot.context.jobId)
  useEffect(() => {
    if (job.job && isTerminal(job.job))
      actor.send({ type: 'JOB_TERMINAL', scopeKey, jobId: job.job.id })
  }, [job.job, actor, scopeKey])
  const notified = useRef('')
  useEffect(() => {
    const jobId = snapshot.context.jobId
    if (jobId && notified.current !== jobId) {
      notified.current = jobId
      onStarted(jobId)
    }
  }, [snapshot.context.jobId, onStarted])
  const busy = snapshot.hasTag('busy')
  const quote = snapshot.context.quote
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <Button
        variant="cta"
        disabled={
          !available || busy || snapshot.matches('started') || snapshot.matches('uncertain')
        }
        pending={snapshot.matches('quoting')}
        aria-describedby={reason ? reasonId : undefined}
        onClick={() => {
          if (available && selected)
            actor.send({ type: 'ESTIMATE', scopeKey, model: { ...selected }, versionKey })
        }}
      >
        {profile.made ? t('make.again') : t('make.first')}
      </Button>
      {reason && (
        <Typography variant="label" as="p" id={reasonId} className="min-w-0">
          {reason}{' '}
          {noModel && ready && !profile.voice.deleted && (
            <Link
              to="/ai-models"
              className={typographyStyles({
                variant: 'label',
                className: 'text-link-fg hover:text-link-fg-hover underline',
              })}
            >
              {t('make.chooseModel')}
            </Link>
          )}
        </Typography>
      )}
      {snapshot.context.failure && (
        <FieldMessage className="w-full">
          <AppFailureMessage failure={snapshot.context.failure} />
        </FieldMessage>
      )}
      {(snapshot.matches('uncertain') || snapshot.matches('checking')) && (
        <>
          <Button
            variant="secondary"
            disabled={busy}
            pending={busy}
            onClick={() => actor.send({ type: 'RECHECK', scopeKey })}
          >
            {t('make.checkState')}
          </Button>
          <Typography variant="label" as="p" role="status">
            {t('make.uncertain')}
          </Typography>
        </>
      )}
      <Dialog
        open={snapshot.matches('quoted') || snapshot.matches('starting')}
        title={t(profile.made ? 'make.confirmAgain' : 'make.confirmFirst')}
        confirmLabel={t(profile.made ? 'make.startAgain' : 'make.startFirst')}
        pending={snapshot.matches('starting')}
        onClose={() => actor.send({ type: 'CANCEL', scopeKey })}
        onConfirm={() => actor.send({ type: 'CONFIRM', scopeKey, versionKey, available })}
      >
        <Typography variant="label" as="p" className="mb-2 break-words">
          {t('make.model', {
            model: `${snapshot.context.model?.providerId ?? ''}/${snapshot.context.model?.modelId ?? ''}`,
          })}
        </Typography>
        <Typography variant="body" as="p">
          {quote?.free
            ? t('make.free')
            : quote?.credits !== undefined
              ? t('make.credits', { credits: quote.credits })
              : t('make.estimateFailed')}
        </Typography>
        <Typography variant="body" as="p" className="mt-2">
          {t('make.previous')}
        </Typography>
      </Dialog>
    </div>
  )
}
