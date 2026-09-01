import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import type { LeaderboardEntry } from '@/entities/model-experiment'
import { formatNumber } from '@/shared/lib'
import { Badge, Typography } from '@/shared/ui'

export function ModelLeaderboard({ entries }: { entries: LeaderboardEntry[] }) {
  const { t } = useTranslation('models')
  if (entries.length === 0)
    return (
      <Typography variant="body" className="text-content-tertiary">
        {t('leaderboard.empty')}
      </Typography>
    )
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
          <Typography variant="label" className="text-content-tertiary">
            #{entry.rank}
          </Typography>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              {/* `min-w-0`: without it a flex item's automatic minimum size is its min-content
                  width, and `truncate` sets `white-space: nowrap` — so min-content is the ENTIRE
                  label and the ellipsis can never fire. A model registered without a label falls
                  back to its id, which overflows the page into horizontal scroll (§8.5). */}
              <Typography variant="label" className="text-content-primary min-w-0 truncate">
                {entry.modelLabel || t('unavailable')}
              </Typography>
              {entry.provisional && <Badge>{t('leaderboard.collecting')}</Badge>}
              {entry.active && <Badge tone="success">{t('leaderboard.active')}</Badge>}
              {entry.recommended && <Badge tone="info">{t('leaderboard.recommended')}</Badge>}
              {entry.disappeared && <Badge tone="warning">{t('leaderboard.disappeared')}</Badge>}
            </div>
            <Typography variant="meta" as="p" className="mt-1">
              {t('leaderboard.record', {
                matches: entry.matches,
                wins: entry.wins,
                losses: entry.losses,
                rate: formatNumber(entry.winRate * 100, undefined, {
                  maximumFractionDigits: 0,
                }),
              })}
            </Typography>
            {/* The accounting detail is the desktop's; on a phone the record and the rating are
                what the board is read for, and this second line doubled the height of every row. */}
            <Typography variant="meta" as="p" className="mt-1 hidden sm:block">
              {t('leaderboard.metrics', {
                calls: formatNumber(entry.successfulCalls),
                latency: formatNumber(entry.averageLatencyMs),
                prompt: formatNumber(entry.promptTokens),
                completion: formatNumber(entry.completionTokens),
                cost: costLabel(entry, t),
              })}
            </Typography>
          </div>
          <Typography variant="label" as="span" className="text-content-primary whitespace-nowrap">
            Elo {entry.rating}
          </Typography>
        </li>
      ))}
    </ol>
  )
}

function costLabel(entry: LeaderboardEntry, t: TFunction<'models'>): string {
  if (entry.costQuality === 'unavailable') return t('leaderboard.costUnavailable')
  const prefix =
    entry.costQuality === 'estimated'
      ? '≈ '
      : entry.costQuality === 'mixed'
        ? t('leaderboard.partlyEstimated')
        : ''
  return `${prefix}${formatNumber(Number(entry.totalCostMicrousd) / 1_000_000, undefined, {
    style: 'currency',
    currency: 'USD',
    currencyDisplay: 'narrowSymbol',
    minimumFractionDigits: 6,
    maximumFractionDigits: 6,
  })}`
}
