import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from '@tanstack/react-router'
import { useExperiment } from '@/entities/model-experiment'
import { useSession } from '@/entities/session'
import { useVoices, voiceRefLabel } from '@/entities/voice'
import { ExperimentActions, hasExperimentActions } from '@/features/review-model-experiment'
import {
  CandidateComparison,
  candidateSides,
  type CandidateSide,
} from '@/widgets/candidate-comparison'
import {
  ActionBar,
  Badge,
  Button,
  SegmentedControl,
  Typography,
  typographyStyles,
  pageStyles,
} from '@/shared/ui'

export function ModelExperimentPage() {
  const { t } = useTranslation(['models', 'common'])
  const { id } = useParams({ from: '/authenticated/ai-models/experiments/$id' })
  const { experiment, isPending, isError, refetch } = useExperiment(id)
  const [activeCandidateId, setActiveCandidateId] = useState('')
  if (isPending) return <Placeholder>{t('experiment.loading', { ns: 'models' })}</Placeholder>
  if (isError || !experiment)
    return (
      <Placeholder>
        <p>{t('experiment.loadFailed', { ns: 'models' })}</p>
        <Button variant="ghost" className="mt-4" onClick={() => void refetch()}>
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Placeholder>
    )
  const sides = candidateSides(experiment.candidates)
  const activeId = activeSideId(sides, activeCandidateId)
  return (
    <main className={pageStyles({ width: 'board' })}>
      {/* A comparison started from a post is a detour from that post, so "back" returns there —
          not to the AI 모델 page the user never visited. Only a post-less (analyze) experiment
          belongs to AI 모델. */}
      {experiment.postSlug ? (
        <Link
          to="/posts/$slug"
          params={{ slug: experiment.postSlug }}
          className={typographyStyles({
            variant: 'label',
            className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center',
          })}
        >
          {t('experiment.backPost', { ns: 'models' })}
        </Link>
      ) : (
        <Link
          to="/ai-models"
          className={typographyStyles({
            variant: 'label',
            className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center',
          })}
        >
          {t('experiment.backModels', { ns: 'models' })}
        </Link>
      )}
      <Typography variant="display" className="mt-4">
        {t('experiment.title', { ns: 'models' })}
      </Typography>
      {/* Desktop-only: on a phone this static instruction costs ~90px — four lines of the candidate
          text the screen exists to show — every single visit, and the A/B switch plus the 후보 A/B
          headings already carry what it says (§0). */}
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
          purpose says today. */}
      {experiment.purposeName && (
        <Typography variant="label" as="p" className="mt-2 break-words">
          {t('experiment.purpose', { ns: 'models', name: experiment.purposeName })}
        </Typography>
      )}
      <div className="mt-6 sm:mt-8">
        <CandidateComparison experiment={experiment} activeCandidateId={activeId} />
      </div>
      {activeId && (
        <ActionBar
          ariaLabel={t('experiment.actionAria', { ns: 'models' })}
          // With no action left to offer, the dock exists only to carry the phone's A/B switch —
          // which the `md:` two-pane layout does not render, so there it would be an empty slab.
          className={hasExperimentActions(experiment) ? undefined : 'md:hidden'}
        >
          <div className="grid gap-3">
            {/* This remains visible at every breakpoint. Phones use it to switch the one visible
                panel; desktop uses it to identify which of the two visible panels the decision
                button will choose. Hiding it on desktop made candidate B impossible to select. */}
            <SegmentedControl
              value={activeId}
              options={sides.map(({ candidate, label }) => ({ value: candidate.id, label }))}
              onChange={setActiveCandidateId}
              ariaLabel={t('experiment.selectAria', { ns: 'models' })}
            />
            <ExperimentActions experiment={experiment} activeCandidateId={activeId} />
          </div>
        </ActionBar>
      )}
    </main>
  )
}

/** Which voice an analyze comparison froze — and, once it is decided, the only voice the winner
 *  can be applied to. Named even after that voice is deleted, so the record stays legible. */
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

function Placeholder({ children }: { children: ReactNode }) {
  return (
    <main className={pageStyles({ className: 'py-10' })}>
      {/* One live region for both the loading and the failed copy: the two branches swap the text
          inside this same node, so the failure is announced as a change instead of silently
          replacing the pending state, which was never announced at all (§9). `py-10` keeps the
          retry button — the only way out of a failed load — within reach on a tall phone (§4.3). */}
      <Typography variant="body" as="div" role="status" className="text-content-tertiary">
        {children}
      </Typography>
    </main>
  )
}
