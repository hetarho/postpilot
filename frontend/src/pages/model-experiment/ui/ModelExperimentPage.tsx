import { useState, type ReactNode } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { useExperiment } from '@/entities/model-experiment'
import { ExperimentActions } from '@/features/review-model-experiment'
import { CandidateComparison } from '@/widgets/candidate-comparison'
import { Button } from '@/shared/ui'

export function ModelExperimentPage() {
  const { id } = useParams({ from: '/authenticated/ai-models/experiments/$id' })
  const { experiment, isPending, isError, refetch } = useExperiment(id)
  const [activeCandidateId, setActiveCandidateId] = useState('')
  if (isPending) return <Placeholder>비교 결과를 불러오는 중…</Placeholder>
  if (isError || !experiment)
    return (
      <Placeholder>
        <span role="alert">비교 결과를 불러오지 못했어요.</span>
        <Button variant="ghost" className="mt-4 underline" onClick={() => void refetch()}>
          다시 시도
        </Button>
      </Placeholder>
    )
  return (
    <main className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
      <Link
        to="/ai-models"
        className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
      >
        ← AI 모델
      </Link>
      <h1 className="mt-4 text-2xl font-semibold tracking-tight">블라인드 비교</h1>
      <p className="text-content-secondary mt-2 text-sm">
        선택하기 전에는 모델 이름과 비용을 숨깁니다. 좌우 후보는 다시 열어도 바뀌지 않습니다.
      </p>
      <div className="mt-8">
        <CandidateComparison
          experiment={experiment}
          activeCandidateId={selectedCandidateID(experiment.candidates, activeCandidateId)}
          onActiveCandidateChange={setActiveCandidateId}
        />
      </div>
      {selectedCandidateID(experiment.candidates, activeCandidateId) && (
        <ExperimentActions
          experiment={experiment}
          activeCandidateId={selectedCandidateID(experiment.candidates, activeCandidateId)}
        />
      )}
    </main>
  )
}

function selectedCandidateID(candidates: Array<{ id: string }>, selected: string): string {
  return candidates.some((candidate) => candidate.id === selected)
    ? selected
    : (candidates[0]?.id ?? '')
}

function Placeholder({ children }: { children: ReactNode }) {
  return (
    <main className="text-content-tertiary mx-auto w-full max-w-2xl px-4 py-16 text-sm sm:px-6">
      {children}
    </main>
  )
}
