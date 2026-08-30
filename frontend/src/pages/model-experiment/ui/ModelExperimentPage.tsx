import { useState, type ReactNode } from 'react'
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
import { ActionBar, Button, SegmentedControl } from '@/shared/ui'

export function ModelExperimentPage() {
  const { id } = useParams({ from: '/authenticated/ai-models/experiments/$id' })
  const { experiment, isPending, isError, refetch } = useExperiment(id)
  const [activeCandidateId, setActiveCandidateId] = useState('')
  if (isPending) return <Placeholder>비교 결과를 불러오는 중…</Placeholder>
  if (isError || !experiment)
    return (
      <Placeholder>
        <p>비교 결과를 불러오지 못했어요.</p>
        <Button variant="ghost" className="mt-4" onClick={() => void refetch()}>
          다시 시도
        </Button>
      </Placeholder>
    )
  const sides = candidateSides(experiment.candidates)
  const activeId = activeSideId(sides, activeCandidateId)
  return (
    <main className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
      {/* A comparison started from a post is a detour from that post, so "back" returns there —
          not to the AI 모델 page the user never visited. Only a post-less (analyze) experiment
          belongs to AI 모델. */}
      {experiment.postSlug ? (
        <Link
          to="/posts/$slug"
          params={{ slug: experiment.postSlug }}
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
        >
          ← 글로 돌아가기
        </Link>
      ) : (
        <Link
          to="/ai-models"
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
        >
          ← AI 모델
        </Link>
      )}
      <h1 className="mt-4 text-2xl font-semibold tracking-tight">블라인드 비교</h1>
      {/* Desktop-only: on a phone this static instruction costs ~90px — four lines of the candidate
          text the screen exists to show — every single visit, and the A/B switch plus the 후보 A/B
          headings already carry what it says (§0). */}
      <p className="text-content-secondary mt-2 hidden text-sm sm:block">
        선택하기 전에는 모델 이름과 비용을 숨깁니다. 좌우 후보는 다시 열어도 바뀌지 않습니다.
      </p>
      {experiment.voiceId && <ExperimentVoice voiceId={experiment.voiceId} />}
      <div className="mt-6 sm:mt-8">
        <CandidateComparison experiment={experiment} activeCandidateId={activeId} />
      </div>
      {activeId && (
        <ActionBar
          ariaLabel="후보 전환과 결정"
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
              ariaLabel="선택할 후보"
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
  const { user } = useSession()
  const { voices } = useVoices(user?.id ?? '')
  const voice = voices.find((candidate) => candidate.id === voiceId)
  return (
    <p className="text-content-secondary mt-2 text-sm break-words">
      말투 · {voice ? voiceRefLabel(voice) : voiceId}
    </p>
  )
}

function activeSideId(sides: CandidateSide[], selected: string): string {
  return sides.some(({ candidate }) => candidate.id === selected)
    ? selected
    : (sides[0]?.candidate.id ?? '')
}

function Placeholder({ children }: { children: ReactNode }) {
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-10 sm:px-6">
      {/* One live region for both the loading and the failed copy: the two branches swap the text
          inside this same node, so the failure is announced as a change instead of silently
          replacing the pending state, which was never announced at all (§9). `py-10` keeps the
          retry button — the only way out of a failed load — within reach on a tall phone (§4.3). */}
      <div role="status" className="text-content-tertiary text-sm">
        {children}
      </div>
    </main>
  )
}
