import { useMemo } from 'react'
import type {
  CandidateStatusName,
  ExperimentCandidate,
  ModelExperiment,
} from '@/entities/model-experiment'
import { Badge, Notice, type BadgeTone } from '@/shared/ui'
import { candidateSides, type CandidateSide } from '../model/sides'

interface CandidateComparisonProps {
  experiment: ModelExperiment
  activeCandidateId: string
}

const STATUS_LABELS: Record<CandidateStatusName, string> = {
  pending: '생성 중',
  running: '생성 중',
  succeeded: '완료',
  failed: '오류',
}

const STATUS_TONES: Record<CandidateStatusName, BadgeTone> = {
  pending: 'neutral',
  running: 'neutral',
  succeeded: 'success',
  failed: 'danger',
}

/** The panels only. The A/B switch is docked by the page instead: it is pressed on every pass of
 *  the comparison, so it belongs in the same thumb band as the buttons that commit it, not pinned
 *  to the top edge ~700px away from the resting thumb (design-language §4.3). */
export function CandidateComparison({ experiment, activeCandidateId }: CandidateComparisonProps) {
  const sides = useMemo(() => candidateSides(experiment.candidates), [experiment.candidates])
  return (
    <div>
      <div className="grid gap-4 md:grid-cols-2">
        {sides.map(({ candidate, label }) => (
          <article
            key={candidate.id}
            aria-label={`후보 ${label}`}
            // The panel is NOT its own scroll container. A phone screen has one scroller, the
            // document (§4.4), and the inner one lost the reader's place on every switch because
            // `hidden` resets its scrollTop — paragraph-by-paragraph comparison, the whole point of
            // the screen, was impossible. The card is a `md:` treatment for the same reason: below
            // that only one panel is on screen, so a raised box around it frames the entire page
            // and costs two Korean characters per line of gutter (§1.4).
            className={`${candidate.id === activeCandidateId ? 'block' : 'hidden'} md:bg-surface-raised md:block md:rounded-lg md:p-4`}
          >
            <div className="flex min-h-11 items-center justify-between gap-2">
              <h2 className="text-lg font-semibold tracking-tight">후보 {label}</h2>
              <Badge tone={STATUS_TONES[candidate.status]}>{STATUS_LABELS[candidate.status]}</Badge>
            </div>
            <CandidateOutput candidate={candidate} />
          </article>
        ))}
      </div>
      {experiment.revealed && <RevealBand sides={sides} />}
    </div>
  )
}

function CandidateOutput({ candidate }: { candidate: ExperimentCandidate }) {
  if (candidate.status === 'failed')
    return (
      <Notice tone="danger" role="alert" className="mt-4">
        {candidate.error || '결과를 만들지 못했어요.'}
      </Notice>
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
              // `break-words`: the filename comes from the server (§3.2).
              <p
                key={index}
                className="bg-surface-recessed rounded-md px-3 py-2 text-sm break-words"
              >
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
            <dt className="text-sm font-medium break-words">{item.file}</dt>
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

/** Once the blind is lifted, both models' identity and accounting sit in ONE band below the
 *  panels. Inside the panels they could only ever be read one at a time below `md:`, at the very
 *  bottom of a post-length column — so comparing the two costs, the payoff of the whole exercise,
 *  meant memorising one number and switching (design-language §4.3). */
function RevealBand({ sides }: { sides: CandidateSide[] }) {
  return (
    <dl className="bg-surface-recessed divide-divider mt-6 divide-y rounded-lg px-4">
      {sides.map(({ candidate, label }) => (
        <div key={candidate.id} className="py-4">
          <dt className="flex flex-wrap items-baseline gap-x-2">
            <span className="text-content-tertiary text-sm">후보 {label}</span>
            <span className="min-w-0 text-sm font-medium">
              {candidate.modelLabel || '등록 해제된 모델'}
            </span>
          </dt>
          <dd className="mt-1">
            {/* `text-sm`, not the metadata role: after the reveal this is the most important
                content on the screen (§3). */}
            <p className="text-content-secondary text-sm">{usageLine(candidate)}</p>
            {candidate.model && (
              <p className="text-content-tertiary mt-1 text-xs break-words">
                {candidate.model.providerId}/{candidate.model.modelId}
              </p>
            )}
          </dd>
        </div>
      ))}
    </dl>
  )
}

function usageLine(candidate: ExperimentCandidate): string {
  const usage = candidate.usage
  if (!usage) return '사용량 미제공'
  const cost =
    usage.costSource === 'unavailable'
      ? '비용 미제공'
      : `${usage.costSource === 'estimated' ? '≈ ' : ''}$${(Number(usage.costMicrousd) / 1_000_000).toFixed(6)}`
  return `${usage.promptTokens.toLocaleString()} 입력 · ${usage.completionTokens.toLocaleString()} 출력 · ${usage.latencyMs.toLocaleString()}ms · ${cost}`
}
