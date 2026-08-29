import type { LeaderboardEntry } from '@/entities/model-experiment'
import { Badge } from '@/shared/ui'

export function ModelLeaderboard({ entries }: { entries: LeaderboardEntry[] }) {
  if (entries.length === 0)
    return <p className="text-content-tertiary text-sm">아직 비교 결과가 없어요.</p>
  return (
    <ol className="divide-divider divide-y">
      {entries.map((entry) => (
        <li
          key={`${entry.model.providerId}/${entry.model.modelId}`}
          // Three columns at the BASE breakpoint, not from `sm:` (§1.5). Stacked, an entry was
          // ~192px tall and put the two values a leaderboard exists to show — the rank and the
          // rating — ~100px apart with the metrics wedged between them. At 360px '#10' and
          // 'Elo 1516' cost ~90px together, which leaves the label and its badges room to wrap.
          className="grid grid-cols-[auto_1fr_auto] items-baseline gap-x-3 py-4"
        >
          <span className="text-content-tertiary text-sm">#{entry.rank}</span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              {/* `min-w-0`: without it a flex item's automatic minimum size is its min-content
                  width, and `truncate` sets `white-space: nowrap` — so min-content is the ENTIRE
                  label and the ellipsis can never fire. A model registered without a label falls
                  back to its id, which overflows the page into horizontal scroll (§8.5). */}
              <span className="min-w-0 truncate text-sm font-medium">
                {entry.modelLabel || '등록 해제된 모델'}
              </span>
              {entry.provisional && <Badge>데이터 수집 중</Badge>}
              {entry.active && <Badge tone="success">활성</Badge>}
              {entry.recommended && <Badge tone="info">추천</Badge>}
              {entry.disappeared && <Badge tone="warning">등록 해제</Badge>}
            </div>
            <p className="text-content-tertiary mt-1 text-xs">
              {entry.matches}전 {entry.wins}승 {entry.losses}패 · 승률{' '}
              {(entry.winRate * 100).toFixed(0)}%
            </p>
            {/* The accounting detail is the desktop's; on a phone the record and the rating are
                what the board is read for, and this second line doubled the height of every row. */}
            <p className="text-content-tertiary mt-1 hidden text-xs sm:block">
              성공 호출 {entry.successfulCalls} · 평균 {entry.averageLatencyMs.toLocaleString()}ms ·
              토큰 {entry.promptTokens.toLocaleString()} / {entry.completionTokens.toLocaleString()}{' '}
              · {costLabel(entry)}
            </p>
          </div>
          <strong className="text-sm whitespace-nowrap">Elo {entry.rating}</strong>
        </li>
      ))}
    </ol>
  )
}

function costLabel(entry: LeaderboardEntry): string {
  if (entry.costQuality === 'unavailable') return '비용 미제공'
  const prefix =
    entry.costQuality === 'estimated' ? '≈ ' : entry.costQuality === 'mixed' ? '일부 ≈ ' : ''
  return `${prefix}$${(Number(entry.totalCostMicrousd) / 1_000_000).toFixed(6)}`
}
