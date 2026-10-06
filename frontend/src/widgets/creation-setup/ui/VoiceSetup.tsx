import { useLatestWritingVoiceCandidates } from '@/entities/voice-candidate'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Sparkles, MessageCircle, ArrowLeft } from 'lucide-react'
import {
  useCreateVoice,
  useVoiceProfile,
  useVoiceAnalysisQueryKey,
  useSetDefaultVoice,
  VOICE_NAME_MAX_CHARS,
  type Voice,
} from '@/entities/voice'
import { useJob, isTerminal, ProgressLine, FailureNotice } from '@/entities/generation-job'
import type { SetupController } from '@/features/complete-setup'
import { MakeVoiceButton } from '@/features/make-voice'
import { PasteMaterialSheet } from '@/features/paste-voice-material'
import { VoiceQuestionnaire } from '@/features/answer-voice-prompt'
import { GeneratedWritingVoices } from '@/features/generate-writing-voices'
import { Button, FieldMessage, Notice, Typography } from '@/shared/ui'

type LearningMethod = 'auto' | 'choose' | 'questions' | 'ai'
export function VoiceSetup({
  ownerId,
  controller,
}: {
  ownerId: string
  controller: SetupController
}) {
  const { t } = useTranslation('creation')
  const unfinished = controller.availability.voices.active.find((voice) => !voice.made)
  const [voiceId, setVoiceId] = useState(unfinished?.id ?? '')
  const [method, setMethod] = useState<LearningMethod>(unfinished ? 'questions' : 'auto')
  const priorCandidates = useLatestWritingVoiceCandidates(method === 'auto' ? ownerId : '')
  const activeMethod = method === 'auto' ? (priorCandidates.batch?.jobId ? 'ai' : 'choose') : method
  const [startedJobId, setStartedJobId] = useState('')
  const [confirmationFailed, setConfirmationFailed] = useState(false)
  const [createFailed, setCreateFailed] = useState(false)
  const create = useCreateVoice(ownerId)
  const makeDefault = useSetDefaultVoice(ownerId)
  const query = useVoiceProfile(ownerId, voiceId)
  const profileKey = useVoiceAnalysisQueryKey(ownerId, voiceId)
  const invalidation = useMemo(() => [profileKey], [profileKey])
  const jobId = startedJobId || query.profile?.activeJobId || ''
  const job = useJob(jobId, invalidation)
  const runningOperation = useRef<number | null>(null)
  const childOperation = useRef<number | null>(null)
  const knownJob = useRef('')
  const { begin, running, success, failure } = controller
  const busy = controller.state.phase === 'saving' || controller.state.phase === 'running'
  const preparePersonal = async () => {
    if (voiceId) {
      setMethod('questions')
      return
    }
    const operation = begin('voice')
    if (operation === null) return
    setCreateFailed(false)
    const base = t('setup.voice.autoName')
    const names = new Set(controller.availability.voices.active.map((voice) => voice.name))
    let name = base,
      index = 2
    while (names.has(name)) {
      const suffix = ` ${index++}`
      name =
        Array.from(base)
          .slice(0, VOICE_NAME_MAX_CHARS - suffix.length)
          .join('') + suffix
    }
    try {
      const result = await create.create({ name })
      if (!result.voice?.id) throw new Error('Missing confirmed writing voice')
      setVoiceId(result.voice.id)
      setMethod('questions')
      success('voice', operation, false)
    } catch {
      setCreateFailed(true)
      failure('voice', operation)
    }
  }
  useEffect(() => {
    if (!jobId || knownJob.current === jobId) return
    knownJob.current = jobId
    const operation = begin('voice')
    if (operation !== null) {
      runningOperation.current = operation
      running('voice', operation)
    }
  }, [jobId, begin, running])
  useEffect(() => {
    const operation = runningOperation.current
    if (operation === null) return
    if (query.profile?.made && !query.profile.activeJobId) success('voice', operation, false)
    else if (job.job && isTerminal(job.job) && job.job.status !== 'done')
      failure('voice', operation)
    else return
    runningOperation.current = null
  }, [job.job, query.profile?.made, query.profile?.activeJobId, success, failure])
  const onChildBusy = (pending: boolean) => {
    if (pending && childOperation.current === null) {
      const operation = begin('voice')
      if (operation !== null) {
        childOperation.current = operation
        running('voice', operation)
      }
    } else if (!pending && childOperation.current !== null) {
      success('voice', childOperation.current, false)
      childOperation.current = null
    }
  }
  const adopted = (voice: Voice) => {
    if (!voice.made || voice.deleted) return
    const operation = childOperation.current
    childOperation.current = null
    if (operation !== null) success('voice', operation, true)
    else controller.next(true)
  }
  const confirmVoice = async () => {
    if (!query.profile?.made) return
    const operation = begin('voice')
    if (operation === null) return
    setConfirmationFailed(false)
    try {
      const response = await makeDefault.setDefault(voiceId)
      if (!response.voices.some((voice) => voice.id === voiceId && voice.isDefault && voice.made))
        throw new Error('Unconfirmed writing voice')
      success('voice', operation, true)
    } catch {
      setConfirmationFailed(true)
      failure('voice', operation)
    }
  }
  const make = (close: () => void): ReactNode => (
    <MakeVoiceButton
      ownerId={ownerId}
      voiceId={voiceId}
      profile={query.profile!}
      onStarted={(id) => {
        close()
        setStartedJobId(id)
      }}
    />
  )
  if (activeMethod === 'choose')
    return (
      <div className="space-y-4">
        <Button
          variant="cta"
          className="w-full justify-start gap-3"
          disabled={busy}
          pending={create.isPending}
          onClick={() => void preparePersonal()}
        >
          <MessageCircle aria-hidden="true" className="size-5 shrink-0" />
          {t('setup.voice.questionPath')}
        </Button>
        <Typography variant="body" className="text-content-secondary">
          {t('setup.voice.questionPathHelp')}
        </Typography>
        <Button
          variant="secondary"
          className="w-full justify-start gap-3"
          disabled={busy}
          onClick={() => setMethod('ai')}
        >
          <Sparkles aria-hidden="true" className="size-5 shrink-0" />
          {t('setup.voice.aiPath')}
        </Button>
        <Typography variant="body" className="text-content-secondary">
          {t('setup.voice.aiPathHelp')}
        </Typography>
        {createFailed && (
          <FieldMessage>{create.errorMessage || t('setup.voice.createFailed')}</FieldMessage>
        )}
      </div>
    )
  if (activeMethod === 'ai')
    return (
      <div className="space-y-6">
        <GeneratedWritingVoices ownerId={ownerId} onAdopted={adopted} onBusyChange={onChildBusy} />
        <Button variant="ghost" disabled={busy} onClick={() => setMethod('choose')}>
          <ArrowLeft aria-hidden="true" className="size-4" />
          {t('setup.voice.otherMethod')}
        </Button>
      </div>
    )
  if (query.isError)
    return (
      <Notice tone="danger" role="alert">
        {t('setup.loadFailed')}
        <Button variant="ghost" onClick={query.refetch}>
          {t('setup.retry')}
        </Button>
      </Notice>
    )
  if (!query.profile)
    return (
      <Typography variant="body" role="status">
        {t('setup.checking')}
      </Typography>
    )
  const profile = query.profile
  return (
    <div className="space-y-6">
      {profile.made ? (
        <>
          <Notice tone="success" role="status">
            {t('setup.voice.confirmed')}
          </Notice>
          <Button
            variant="cta"
            className="w-full"
            disabled={busy}
            pending={makeDefault.isPending}
            onClick={() => void confirmVoice()}
          >
            {t('setup.voice.use')}
          </Button>
          {(makeDefault.isError || confirmationFailed) && (
            <FieldMessage>{makeDefault.errorMessage || t('setup.voice.createFailed')}</FieldMessage>
          )}
        </>
      ) : busy && jobId ? (
        <Typography variant="body" role="status">
          {t('setup.voice.running')}
        </Typography>
      ) : (
        <VoiceQuestionnaire
          ownerId={ownerId}
          voiceId={voiceId}
          samples={profile.samples}
          profile={profile}
          renderMakeVoice={make}
          onBusyChange={onChildBusy}
        />
      )}
      <div role="status">
        {jobId && job.job && !isTerminal(job.job) && <ProgressLine job={job.job} />}
        {jobId && job.isError && (
          <FailureNotice message={t('setup.voice.failed')} onRetry={job.refetch} />
        )}
        {job.job?.status === 'failed' && <FailureNotice failure={job.job.failure} />}
      </div>
      {!busy && !profile.made && (
        <details>
          <summary className="min-h-11 py-3">
            <Typography variant="body" as="span">
              {t('setup.voice.moreWays')}
            </Typography>
          </summary>
          <div className="mt-3 flex flex-wrap gap-3">
            <PasteMaterialSheet ownerId={ownerId} voiceId={voiceId} />
            <Button variant="ghost" onClick={() => setMethod('choose')}>
              {t('setup.voice.otherMethod')}
            </Button>
          </div>
        </details>
      )}
    </div>
  )
}
