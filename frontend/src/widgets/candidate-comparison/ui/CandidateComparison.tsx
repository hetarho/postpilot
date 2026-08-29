import { useMemo } from 'react'
import type { ExperimentCandidate, ModelExperiment } from '@/entities/model-experiment'
import { Badge, SegmentedControl } from '@/shared/ui'

interface CandidateComparisonProps {
  experiment: ModelExperiment
  activeCandidateId: string
  onActiveCandidateChange: (id: string) => void
}

export function CandidateComparison({
  experiment,
  activeCandidateId,
  onActiveCandidateChange,
}: CandidateComparisonProps) {
  const candidates = useMemo(
    () => [...experiment.candidates].sort((a, b) => a.displaySide.localeCompare(b.displaySide)),
    [experiment.candidates],
  )
  const options = candidates.map((candidate, index) => ({
    value: candidate.id,
    label: index === 0 ? 'A' : 'B',
  }))
  return (
    <div>
      <SegmentedControl
        value={activeCandidateId}
        options={options}
        onChange={onActiveCandidateChange}
        ariaLabel="비교 후보"
        className="sticky top-2 z-10 md:hidden"
      />
      <div className="mt-4 grid gap-4 md:grid-cols-2">
        {candidates.map((candidate, index) => (
          <article
            key={candidate.id}
            aria-label={`후보 ${index === 0 ? 'A' : 'B'}`}
            className={`${candidate.id === activeCandidateId ? 'block' : 'hidden'} bg-surface-raised max-h-[65vh] overflow-y-auto rounded-lg p-4 md:block md:max-h-none md:overflow-visible`}
          >
            <div className="flex min-h-11 items-center justify-between gap-2">
              <h2 className="text-lg font-semibold tracking-tight">
                후보 {index === 0 ? 'A' : 'B'}
              </h2>
              <Badge>
                {candidate.status === 'succeeded'
                  ? '완료'
                  : candidate.status === 'failed'
                    ? '오류'
                    : '생성 중'}
              </Badge>
            </div>
            <CandidateOutput candidate={candidate} />
            {experiment.revealed && <Reveal candidate={candidate} />}
          </article>
        ))}
      </div>
    </div>
  )
}

function CandidateOutput({ candidate }: { candidate: ExperimentCandidate }) {
  if (candidate.status === 'failed')
    return (
      <p
        role="alert"
        className="bg-notice-danger-bg text-notice-danger-fg mt-4 rounded-md px-3 py-2 text-sm"
      >
        {candidate.error || '결과를 만들지 못했어요.'}
      </p>
    )
  if (!candidate.output)
    return <p className="text-content-tertiary mt-4 text-sm">결과를 기다리는 중…</p>
  if (candidate.output.kind === 'write') {
    const content = candidate.output.content
    return (
      <div className="mt-4">
        <h3 className="text-lg font-semibold tracking-tight">{content.title}</h3>
        {content.summary && (
          <p className="text-content-secondary mt-2 text-sm">{content.summary}</p>
        )}
        <div className="mt-5 space-y-4">
          {content.blocks.map((block, index) =>
            block.type === 5 ? (
              <ul key={index} className="list-disc space-y-1 pl-5 text-sm leading-relaxed">
                {block.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ) : block.type === 2 ? (
              <h4 key={index} className="font-semibold">
                {block.content}
              </h4>
            ) : block.type === 3 ? (
              <p key={index} className="bg-surface-recessed rounded-md px-3 py-2 text-sm">
                사진 · {block.file}
                {block.caption ? ` — ${block.caption}` : ''}
              </p>
            ) : (
              <p key={index} className="text-sm leading-relaxed whitespace-pre-wrap">
                {block.content}
              </p>
            ),
          )}
        </div>
      </div>
    )
  }
  if (candidate.output.kind === 'observe') {
    return (
      <dl className="divide-divider mt-4 divide-y">
        {candidate.output.observations.map((item) => (
          <div key={item.file} className="py-4">
            <dt className="text-sm font-medium">{item.file}</dt>
            <dd className="text-content-secondary mt-2 space-y-1 text-sm">
              <p>
                <span className="text-content-tertiary">장면</span> {item.scene || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">분위기</span> {item.mood || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">글자</span> {item.visibleText || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">사물</span> {item.objects.join(', ') || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">사람</span>{' '}
                {item.peoplePresent ? '있음' : '없음'}
              </p>
            </dd>
          </div>
        ))}
      </dl>
    )
  }
  return (
    <div className="mt-4 text-sm leading-relaxed whitespace-pre-wrap">
      {candidate.output.styleguide}
    </div>
  )
}

function Reveal({ candidate }: { candidate: ExperimentCandidate }) {
  const usage = candidate.usage
  const cost =
    !usage || usage.costSource === 'unavailable'
      ? '비용 미제공'
      : `${usage.costSource === 'estimated' ? '≈ ' : ''}$${(Number(usage.costMicrousd) / 1_000_000).toFixed(6)}`
  return (
    <div className="bg-surface-recessed text-content-secondary mt-6 rounded-md px-3 py-3 text-xs">
      <p className="font-medium">{candidate.modelLabel || '등록 해제된 모델'}</p>
      {candidate.model && (
        <p className="text-content-tertiary mt-1">
          {candidate.model.providerId}/{candidate.model.modelId}
        </p>
      )}
      <p className="mt-2">
        {usage
          ? `${usage.promptTokens.toLocaleString()} 입력 · ${usage.completionTokens.toLocaleString()} 출력 · ${usage.latencyMs.toLocaleString()}ms · ${cost}`
          : '사용량 미제공'}
      </p>
    </div>
  )
}
