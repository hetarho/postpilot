import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  VoiceReadinessMeter,
  fingerprintRows,
  type VoiceAiField,
  type VoiceAnalysis,
} from '@/entities/voice'
import { MakeVoiceButton } from '@/features/make-voice'
import { RestorePreviousAnalysisButton } from '@/features/restore-voice-analysis'
import { Notice, Typography, typographyStyles } from '@/shared/ui'
import { VoiceRunStatus } from './VoiceRunStatus'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

/** 말투 분석, the voice's first tab (VOICE-63): counted rows without a visible group title,
 *  followed by `AI가 읽은 인상`, each with the owner's own example sentence. */
export function VoiceAnalysisPage() {
  const { t } = useTranslation('nav')
  return (
    <VoiceScreen title={t('voice.analysis')}>
      {(context) => <AnalysisPanel {...context} />}
    </VoiceScreen>
  )
}

function AnalysisPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { t } = useTranslation('voices')
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || profile.activeJobId
  if (!profile.analysis) {
    return (
      <>
        <VoiceReadinessMeter readiness={profile.readiness} />
        <Link
          to="/voices/$voiceId/materials"
          params={{ voiceId }}
          className={typographyStyles({
            variant: 'label',
            className:
              'text-link-fg hover:text-link-fg-hover mt-2 inline-flex min-h-11 items-center underline',
          })}
        >
          {t('screens.toMaterials')}
        </Link>
        <div className="mt-4">
          <MakeVoiceButton
            ownerId={ownerId}
            voiceId={voiceId}
            profile={profile}
            onStarted={setStartedJobId}
          />
        </div>
        <VoiceRunStatus ownerId={ownerId} voiceId={voiceId} jobId={jobId} />
      </>
    )
  }
  const notice =
    profile.notice.kind === 'added'
      ? t('analysis.noticeAdded', { count: profile.notice.count })
      : profile.notice.kind === 'changed'
        ? t('analysis.noticeChanged')
        : ''
  return (
    <>
      {!voice.deleted && (notice || profile.hasPrevious) && (
        <div className="flex flex-col gap-3">
          {notice && (
            <Notice tone="info" role="status">
              <span className="w-full min-w-0">{notice}</span>
              <MakeVoiceButton
                ownerId={ownerId}
                voiceId={voiceId}
                profile={profile}
                onStarted={setStartedJobId}
              />
            </Notice>
          )}
          {profile.hasPrevious && (
            <div>
              <RestorePreviousAnalysisButton ownerId={ownerId} voiceId={voiceId} />
            </div>
          )}
        </div>
      )}
      <VoiceRunStatus ownerId={ownerId} voiceId={voiceId} jobId={jobId} />
      <CountedHabits analysis={profile.analysis} />
      <AiReading analysis={profile.analysis} />
    </>
  )
}

function Quote({ sentence }: { sentence?: string }) {
  if (!sentence) return null
  return (
    <Typography variant="meta" as="p" className="mt-1 break-words">
      “{sentence}”
    </Typography>
  )
}

function CountedHabits({ analysis }: { analysis: VoiceAnalysis }) {
  const { t } = useTranslation('voices')
  return (
    <section aria-label={t('analysis.counted')} className="mt-8">
      <ul className="divide-divider mt-3 divide-y">
        {fingerprintRows(analysis.counted).map((row) => (
          <li key={row.item} className="py-3">
            <Typography variant="label" as="p">
              {row.label}
            </Typography>
            <Typography
              variant="body"
              as="p"
              className={row.unknown ? 'text-content-tertiary mt-1' : 'mt-1 break-words'}
            >
              {row.sentence}
            </Typography>
            <Quote sentence={row.example?.sentence} />
          </li>
        ))}
      </ul>
    </section>
  )
}

function AiReading({ analysis }: { analysis: VoiceAnalysis }) {
  const { t } = useTranslation('voices')
  const exampleOf = (field: VoiceAiField) =>
    analysis.ai.examples.find((example) => example.field === field)?.sentence
  const rows = [
    {
      key: 'impression',
      label: t('analysis.impression'),
      text: analysis.ai.impression,
      field: 'impression' as const,
    },
    {
      key: 'tics',
      label: t('analysis.tics'),
      text: analysis.ai.tics
        .map((tic) =>
          tic.when ? t('analysis.tic', { phrase: tic.phrase, when: tic.when }) : `‘${tic.phrase}’`,
        )
        .join(' · '),
      field: 'tics' as const,
    },
    {
      key: 'phrases',
      label: t('analysis.phrases'),
      text: analysis.ai.signaturePhrases.join(', '),
      field: 'signature_phrases' as const,
    },
  ]
  return (
    <section aria-labelledby="ai-reading" className="mt-8">
      <Typography variant="title" as="h3" id="ai-reading">
        {t('analysis.ai')}
      </Typography>
      <ul className="divide-divider mt-3 divide-y">
        {rows.map((row) => (
          <li key={row.key} className="py-3">
            <Typography variant="label" as="p">
              {row.label}
            </Typography>
            <Typography
              variant="body"
              as="p"
              className={row.text ? 'mt-1 break-words' : 'text-content-tertiary mt-1'}
            >
              {row.text || t('fingerprint.unknown')}
            </Typography>
            <Quote sentence={exampleOf(row.field)} />
          </li>
        ))}
      </ul>
    </section>
  )
}
