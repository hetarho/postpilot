import { Badge, Notice } from '@/shared/ui'

export interface TextComparisonCandidate {
  id: string
  label: string
  text: string
  status: string
  error?: string
}
export function TextCandidateComparison({
  candidates,
  activeCandidateId,
}: {
  candidates: TextComparisonCandidate[]
  activeCandidateId: string
}) {
  const [first, second] = candidates
  return (
    <div className="grid gap-4 md:grid-cols-2">
      {candidates.map((candidate) => (
        <article
          key={candidate.id}
          aria-label={`후보 ${candidate.label}`}
          className={`${candidate.id === activeCandidateId ? 'block' : 'hidden'} md:bg-surface-raised md:block md:rounded-lg md:p-4`}
        >
          <div className="flex min-h-11 items-center justify-between gap-2">
            <h2 className="text-lg font-semibold">후보 {candidate.label}</h2>
            <Badge
              tone={
                candidate.status === 'failed'
                  ? 'danger'
                  : candidate.status === 'succeeded'
                    ? 'success'
                    : 'neutral'
              }
            >
              {candidate.status === 'failed'
                ? '오류'
                : candidate.status === 'succeeded'
                  ? '완료'
                  : '생성 중'}
            </Badge>
          </div>
          {candidate.error ? (
            <Notice tone="danger" className="mt-4">
              {candidate.error}
            </Notice>
          ) : candidate.text ? (
            <p className="mt-4 text-sm leading-relaxed whitespace-pre-wrap">
              {highlight(candidate.text, candidate === first ? second?.text : first?.text)}
            </p>
          ) : (
            <p className="text-content-tertiary mt-4 text-sm">결과를 기다리는 중…</p>
          )}
        </article>
      ))}
    </div>
  )
}
function highlight(text: string, other = '') {
  let start = 0
  while (start < text.length && start < other.length && text[start] === other[start]) start += 1
  let end = 0
  while (
    end < text.length - start &&
    end < other.length - start &&
    text[text.length - 1 - end] === other[other.length - 1 - end]
  )
    end += 1
  if (start === text.length) return text
  return (
    <>
      {text.slice(0, start)}
      <mark className="bg-notice-warning-bg text-notice-warning-fg rounded-sm">
        {text.slice(start, text.length - end)}
      </mark>
      {end ? text.slice(text.length - end) : ''}
    </>
  )
}
