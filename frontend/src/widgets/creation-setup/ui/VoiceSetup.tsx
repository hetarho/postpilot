import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useCreateVoice,
  useVoiceProfile,
  useVoiceAnalysisQueryKey,
  useSetDefaultVoice,
  VoiceReadinessMeter,
  VOICE_NAME_MAX_CHARS,
} from '@/entities/voice'
import { useJob, isTerminal, ProgressLine, FailureNotice } from '@/entities/generation-job'
import type { SetupController } from '@/features/complete-setup'
import { MakeVoiceButton } from '@/features/make-voice'
import { PasteMaterialSheet } from '@/features/paste-voice-material'
import { AnswerPromptsSheet } from '@/features/answer-voice-prompt'
import { Button, FieldLabel, FieldMessage, Notice, TextField, Typography } from '@/shared/ui'

export function VoiceSetup({
  ownerId,
  controller,
}: {
  ownerId: string
  controller: SetupController
}) {
  const { t } = useTranslation('creation')
  const [name, setName] = useState('')
  const [selectedId, setSelectedId] = useState(
    () => controller.availability.voices.active.find((voice) => !voice.made)?.id ?? '',
  )
  const [startedJobId, setStartedJobId] = useState('')
  const [missingResponse, setMissingResponse] = useState(false)
  const [confirmationFailed, setConfirmationFailed] = useState(false)
  const create = useCreateVoice(ownerId)
  const makeDefault = useSetDefaultVoice(ownerId)
  const voiceId = selectedId
  const query = useVoiceProfile(ownerId, voiceId)
  const profileKey = useVoiceAnalysisQueryKey(ownerId, voiceId)
  const invalidation = useMemo(() => [profileKey], [profileKey])
  const jobId = startedJobId || query.profile?.activeJobId || ''
  const job = useJob(jobId, invalidation)
  const runningOperation = useRef<number | null>(null)
  const knownJob = useRef('')
  const { begin, running, success, failure } = controller
  const busy = controller.state.phase === 'saving' || controller.state.phase === 'running'
  const nameChars = Array.from(name.trim()).length
  const createVoice = async (event: FormEvent) => {
    event.preventDefault()
    if (nameChars === 0 || nameChars > VOICE_NAME_MAX_CHARS) return
    const operation = begin('voice')
    if (operation === null) return
    setMissingResponse(false)
    try {
      const result = await create.create({ name: name.trim() })
      if (!result.voice?.id) {
        setMissingResponse(true)
        failure('voice', operation)
        return
      }
      setSelectedId(result.voice.id)
      success('voice', operation, false)
    } catch {
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
    // Profile publication can clear activeJobId before the terminal job query renders again.
    if (query.profile?.made && !query.profile.activeJobId) success('voice', operation, false)
    else if (job.job && isTerminal(job.job) && job.job.status !== 'done')
      failure('voice', operation)
    else return
    runningOperation.current = null
  }, [job.job, query.profile?.made, query.profile?.activeJobId, success, failure])
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
  if (!voiceId)
    return (
      <form onSubmit={(event) => void createVoice(event)}>
        <FieldLabel htmlFor="setup-voice-name">{t('setup.voice.name')}</FieldLabel>
        <TextField
          id="setup-voice-name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder={t('setup.voice.placeholder')}
          autoComplete="off"
          maxLength={VOICE_NAME_MAX_CHARS * 2}
          disabled={busy}
          className="mt-2"
        />
        <div role="status" className="mt-3">
          {(create.isError || missingResponse) && (
            <FieldMessage>{create.errorMessage || t('setup.voice.createFailed')}</FieldMessage>
          )}
        </div>
        <Button
          type="submit"
          variant="cta"
          className="mt-4 w-full"
          disabled={nameChars === 0 || nameChars > VOICE_NAME_MAX_CHARS || busy}
          pending={create.isPending}
        >
          {t('setup.voice.create')}
        </Button>
      </form>
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
      ) : (
        <>
          <Typography variant="title">{t('setup.voice.material')}</Typography>
          <Typography variant="body" className="text-content-secondary">
            {t('setup.voice.materialHelp')}
          </Typography>
          <VoiceReadinessMeter readiness={profile.readiness} />
          <div className="flex flex-wrap gap-3">
            <PasteMaterialSheet ownerId={ownerId} voiceId={voiceId} disabled={busy} />
            <AnswerPromptsSheet
              ownerId={ownerId}
              voiceId={voiceId}
              samples={profile.samples}
              profile={profile}
              disabled={busy}
              renderMakeVoice={(close) => (
                <MakeVoiceButton
                  ownerId={ownerId}
                  voiceId={voiceId}
                  profile={profile}
                  onStarted={(id) => {
                    close()
                    setStartedJobId(id)
                  }}
                />
              )}
            />
          </div>
          {!busy && (
            <MakeVoiceButton
              ownerId={ownerId}
              voiceId={voiceId}
              profile={profile}
              onStarted={setStartedJobId}
            />
          )}
        </>
      )}
      <div role="status">
        {jobId && job.job && !isTerminal(job.job) && <ProgressLine job={job.job} />}
        {jobId && job.isError && (
          <FailureNotice message={t('setup.voice.failed')} onRetry={job.refetch} />
        )}
        {job.job?.status === 'failed' && <FailureNotice failure={job.job.failure} />}
      </div>
    </div>
  )
}
