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
          className="grid gap-2 py-4 sm:grid-cols-[auto_1fr_auto] sm:items-center"
        >
          <span className="text-content-tertiary text-sm">#{entry.rank}</span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="truncate text-sm font-medium">{entry.modelLabel}</span>
              {entry.provisional && <Badge>데이터 수집 중</Badge>}
              {entry.active && <Badge>활성</Badge>}
              {entry.recommended && <Badge>추천</Badge>}
              {entry.disappeared && <Badge>등록 해제</Badge>}
            </div>
            <p className="text-content-tertiary mt-1 text-xs">
              {entry.matches}전 {entry.wins}승 {entry.losses}패 · 승률{' '}
              {(entry.winRate * 100).toFixed(0)}% · 성공 호출 {entry.successfulCalls}
            </p>
            <p className="text-content-tertiary mt-1 text-xs">
              평균 {entry.averageLatencyMs.toLocaleString()}ms · 토큰{' '}
              {entry.promptTokens.toLocaleString()} / {entry.completionTokens.toLocaleString()} ·{' '}
              {costLabel(entry)}
            </p>
          </div>
          <strong className="text-sm">Elo {entry.rating}</strong>
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
