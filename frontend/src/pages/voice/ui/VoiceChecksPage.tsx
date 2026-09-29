import { useId, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { useVoiceChecks, useVoiceChecksQueryKey, type VoiceCheck } from '@/entities/voice'
import { AnswerPromptForm } from '@/features/answer-voice-prompt'
import { RetryCheckButton, StartCheckSheet } from '@/features/check-voice'
import { Badge, Typography } from '@/shared/ui'
import { FingerprintComparison } from '@/widgets/voice-fingerprint'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

/** 검증, the voice's third tab (VOICE-43): `검증하기` on one answered prompt, the running check's
 *  progress, and every result newest first — the owner's answer beside the AI's piece with the
 *  fingerprint comparison, marked when an earlier analysis wrote it, a failure with its retry. */
export function VoiceChecksPage() {
  const { t } = useTranslation('nav')
  return (
    <VoiceScreen title={t('voice.checks')}>{(context) => <ChecksPanel {...context} />}</VoiceScreen>
  )
}

function ChecksPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { t } = useTranslation('voices')
  const { checks, activeJobId, isPending, isError, refetch } = useVoiceChecks(ownerId, voiceId)
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || activeJobId
  // A finished check job refreshes the results (VOICE-55).
  const checksKey = useVoiceChecksQueryKey(ownerId, voiceId)
  const invalidateOnDone = useMemo(() => [checksKey], [checksKey])
  const jobState = useJob(jobId, invalidateOnDone)
  const running = jobId !== '' && !!jobState.job && !isTerminal(jobState.job)
  const blocked = voice.deleted ? t('check.deleted') : !profile.made ? t('check.notMade') : ''

  return (
    <>
      <StartCheckSheet
        ownerId={ownerId}
        voiceId={voiceId}
        samples={profile.samples}
        blocked={blocked}
        busy={running}
        renderAnswer={(prompt, onAnswered, onBack) => (
          <AnswerPromptForm
            ownerId={ownerId}
            voiceId={voiceId}
            prompt={prompt}
            onBack={onBack}
            onDone={onAnswered}
          />
        )}
        onStarted={setStartedJobId}
      />
      {jobId && (
        <section className="mt-6" aria-label={t('checks.status')}>
          {jobState.isError ? (
            <FailureNotice message={t('checks.statusFailed')} onRetry={jobState.refetch} />
          ) : running && jobState.job ? (
            <ProgressLine job={jobState.job} />
          ) : null}
        </section>
      )}
      <div className="mt-8">
        {isError ? (
          <FailureNotice message={t('checks.loadFailed')} onRetry={refetch} />
        ) : isPending ? null : checks.length === 0 ? (
          <Typography variant="body" as="p" className="text-content-secondary">
            {t('checks.empty')}
          </Typography>
        ) : (
          <ol className="flex flex-col">
            {checks.map((check) => (
              <li
                key={check.id}
                className="border-divider border-t py-6 first:border-t-0 first:pt-0"
              >
                <CheckResult
                  ownerId={ownerId}
                  voiceId={voiceId}
                  check={check}
                  retryDisabled={voice.deleted || running}
                  onRetried={setStartedJobId}
                />
              </li>
            ))}
          </ol>
        )}
      </div>
    </>
  )
}

function CheckResult({
  ownerId,
  voiceId,
  check,
  retryDisabled,
  onRetried,
}: {
  ownerId: string
  voiceId: string
  check: VoiceCheck
  retryDisabled: boolean
  onRetried: (jobId: string) => void
}) {
  const { t } = useTranslation('voices')
  const headingId = useId()
  return (
    <article aria-labelledby={headingId}>
      <div className="flex flex-wrap items-center gap-2">
        <Typography variant="fieldTitle" as="h3" id={headingId} className="min-w-0 break-words">
          {check.prompt?.text}
        </Typography>
        {check.stale && <Badge>{t('checks.stale')}</Badge>}
      </div>
      {check.status === 'done' ? (
        <>
          <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Side title={t('checks.answer')}>
              {check.answerDeleted ? (
                <span className="text-content-secondary">{t('checks.answerDeleted')}</span>
              ) : (
                check.answer
              )}
            </Side>
            <Side title={t('checks.piece')}>{check.piece}</Side>
          </div>
          <section className="mt-4" aria-label={t('checks.comparison')}>
            <Typography variant="label" as="h4">
              {t('checks.comparison')}
            </Typography>
            <FingerprintComparison items={check.comparison} textLabel={t('checks.piece')} />
          </section>
        </>
      ) : check.status === 'failed' ? (
        <div className="mt-3 flex flex-col gap-3">
          <FailureNotice failure={check.failure} />
          <RetryCheckButton
            ownerId={ownerId}
            voiceId={voiceId}
            checkId={check.id}
            disabled={retryDisabled}
            onStarted={onRetried}
          />
        </div>
      ) : (
        <Typography variant="body" as="p" className="text-content-secondary mt-3">
          {t('checks.running')}
        </Typography>
      )}
    </article>
  )
}

function Side({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title}>
      <Typography variant="label" as="h4">
        {title}
      </Typography>
      <Typography variant="body" as="p" className="mt-1 break-words whitespace-pre-line">
        {children}
      </Typography>
    </section>
  )
}
