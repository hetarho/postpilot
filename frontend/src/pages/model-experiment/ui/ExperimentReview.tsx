import { useCallback, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  candidateSides,
  legacyOutputText,
  isExperimentActive,
  type ExperimentCandidate,
  useExperiment,
  type CandidateSide,
  type ModelExperiment,
} from '@/entities/model-experiment'
import { useSession } from '@/entities/session'
import { useVoices, voiceRefLabel } from '@/entities/voice'
import { ExperimentActions } from '@/features/review-model-experiment'
import { CandidateComparison } from '@/widgets/candidate-comparison'
import { FingerprintComparison } from '@/widgets/voice-fingerprint'
import { copyText } from '@/shared/lib'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  SegmentedControl,
  Textarea,
  Typography,
  pageStyles,
} from '@/shared/ui'

export function ExperimentReview({
  id,
  backLink,
}: {
  id: string
  backLink: (experiment?: ModelExperiment) => ReactNode
}) {
  const { t } = useTranslation(['models', 'common'])
  const { user } = useSession()
  const { experiment, isPending, isError, refetch } = useExperiment(id, user?.id ?? '')
  const [activeCandidateId, setActiveCandidateId] = useState('')
  const readingPositions = useRef<Record<string, number>>({})
  const previousCandidate = useRef('')
  const comparisonTop = useRef<HTMLDivElement>(null)
  const sides = candidateSides(experiment?.candidates ?? [])
  const activeId = activeSideId(sides, activeCandidateId)
  useLayoutEffect(() => {
    if (
      previousCandidate.current &&
      previousCandidate.current !== activeId &&
      window.matchMedia('(max-width: 767px)').matches
    ) {
      const top = comparisonTop.current
        ? window.scrollY + comparisonTop.current.getBoundingClientRect().top
        : window.scrollY
      window.scrollTo(0, readingPositions.current[activeId] ?? top)
    }
    previousCandidate.current = activeId
  }, [activeId])
  if (isPending)
    return (
      <Placeholder backLink={backLink()}>{t('experiment.loading', { ns: 'models' })}</Placeholder>
    )
  if (isError || !experiment)
    return (
      <Placeholder backLink={backLink()}>
        <p>{t('experiment.loadFailed', { ns: 'models' })}</p>
        <Button variant="ghost" className="mt-4" onClick={() => void refetch()}>
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Placeholder>
    )
  return (
    <main className={pageStyles({ width: 'board' })}>
      {backLink(experiment)}
      <Typography variant="display" className="mt-4">
        {t('experiment.title', { ns: 'models' })}
      </Typography>
      <div className="mt-4">
        <ExperimentActions experiment={experiment} activeCandidateId={activeId} />
      </div>
      {isExperimentActive(experiment.status) && (
        <Notice tone="info" role="status" className="mt-4">
          {t('experiment.legacyRunning', { ns: 'models' })}
        </Notice>
      )}
      {/* Desktop-only: on a phone this static instruction costs ~90px — four lines of the candidate
          text the screen exists to show — every single visit, and the A/B switch plus the 후보 A/B
          headings already carry what it says (THEME-8). */}
      <Typography variant="body" className="text-content-secondary mt-2 hidden sm:block">
        {t('experiment.description', { ns: 'models' })}
      </Typography>
      {experiment.voiceId && <ExperimentVoice voiceId={experiment.voiceId} />}
      {experiment.targetLanguage && (
        <Typography variant="label" as="p" className="mt-2 flex items-center gap-2">
          <span>{t('experiment.language', { ns: 'models' })}</span>
          <Badge>{t(`contentLanguage.${experiment.targetLanguage}`, { ns: 'common' })}</Badge>
        </Typography>
      )}
      {/* Read straight off the frozen snapshot's projection rather than looked up: the brief
          both candidates were given is a property of this comparison, not of whatever the
          template says today. */}
      {experiment.templateName && (
        <Typography variant="label" as="p" className="mt-2 break-words">
          {t('experiment.template', { ns: 'models', name: experiment.templateName })}
        </Typography>
      )}
      <div className="mt-4 flex flex-wrap gap-3">
        {sides.map(({ candidate, label }) => (
          <LegacyCandidateCopy key={candidate.id} candidate={candidate} label={label} />
        ))}
      </div>
      {experiment.candidates.some(
        (candidate) => candidate.status === 'succeeded' && !candidate.output,
      ) && (
        <Notice tone="warning" className="mt-4">
          {t('experiment.legacyExpired', { ns: 'models' })}
        </Notice>
      )}
      <div ref={comparisonTop} className="mt-6 sm:mt-8">
        <CandidateComparison
          experiment={experiment}
          activeCandidateId={activeId}
          // Each 말투 반영 비교 piece carries its fingerprint beside the voice's (MODEL-67).
          renderComparison={(items) => (
            <FingerprintComparison
              items={items}
              textLabel={t('experiment.pieceLabel', { ns: 'models' })}
            />
          )}
        />
      </div>
      {activeId && (
        <ActionBar
          ariaLabel={t('experiment.actionAria', { ns: 'models' })}
          // With no action left to offer, the dock exists only to carry the phone's A/B switch —
          // which the `md:` two-pane layout does not render, so there it would be an empty slab.
          className="md:hidden"
        >
          <div className="grid gap-3">
            {/* Phones switch retained results; desktop reads both panels without a decision dock. */}
            <SegmentedControl
              value={activeId}
              options={sides.map(({ candidate, label }) => ({ value: candidate.id, label }))}
              onChange={(id) => {
                if (activeId) readingPositions.current[activeId] = window.scrollY
                setActiveCandidateId(id)
              }}
              ariaLabel={t('experiment.selectAria', { ns: 'models' })}
            />
          </div>
        </ActionBar>
      )}
    </main>
  )
}

/** Which voice a write comparison froze — the voice both candidates wrote in. Named even after
 *  that voice is deleted, so the record stays legible. */
function ExperimentVoice({ voiceId }: { voiceId: string }) {
  const { t } = useTranslation('models')
  const { user } = useSession()
  const { voices } = useVoices(user?.id ?? '')
  const voice = voices.find((candidate) => candidate.id === voiceId)
  return (
    <Typography variant="label" as="p" className="mt-2 break-words">
      {t('experiment.voice', { ns: 'models', name: voice ? voiceRefLabel(voice) : voiceId })}
    </Typography>
  )
}

function activeSideId(sides: CandidateSide[], selected: string): string {
  return sides.some(({ candidate }) => candidate.id === selected)
    ? selected
    : (sides[0]?.candidate.id ?? '')
}

function Placeholder({ children, backLink }: { children: ReactNode; backLink: ReactNode }) {
  return (
    <main className={pageStyles({ className: 'py-10' })}>
      {backLink}
      {/* One live region for both the loading and the failed copy: the two branches swap the text
          inside this same node, so the failure is announced as a change instead of silently
          replacing the pending state, which was never announced at all (THEME-33). `py-10` keeps the
          retry button and return link within reach on a tall phone (THEME-24). */}
      <Typography variant="body" as="div" role="status" className="text-content-tertiary mt-4">
        {children}
      </Typography>
    </main>
  )
}

function LegacyCandidateCopy({
  candidate,
  label,
}: {
  candidate: ExperimentCandidate
  label: string
}) {
  const { t } = useTranslation('models')
  const [result, setResult] = useState<'idle' | 'copied' | 'fallback'>('idle')
  const selectFallback = useCallback((element: HTMLTextAreaElement | null) => {
    element?.focus()
    element?.select()
  }, [])
  if (candidate.status !== 'succeeded' || !candidate.output) return null
  const text = legacyOutputText(candidate.output)
  return (
    <div className="min-w-0">
      <Button
        variant="ghost"
        onClick={() => {
          void copyText(text).then(({ copied }) => setResult(copied ? 'copied' : 'fallback'))
        }}
      >
        {t('experiment.copyResult', { label })}
      </Button>
      {result === 'copied' && (
        <Typography variant="label" as="p" role="status">
          {t('experiment.copied', { label })}
        </Typography>
      )}
      {result === 'fallback' && (
        <div className="mt-2">
          <Typography variant="label" as="p" role="status">
            {t('experiment.copyFallback')}
          </Typography>
          <Textarea
            readOnly
            aria-label={t('experiment.copyResult', { label })}
            value={text}
            rows={6}
            ref={selectFallback}
          />
        </div>
      )}
    </div>
  )
}
