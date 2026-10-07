import { useTranslation } from 'react-i18next'
import type { LeaderboardEntry, LeaderboardWindowName } from '@/entities/model-experiment'
import { Typography, buttonStyles } from '@/shared/ui'

/** Existing route consumers keep their interface while ranking is replaced by real writing tests. */
export function ModelLeaderboard(props: {
  entries: LeaderboardEntry[]
  window: LeaderboardWindowName
}) {
  void props
  const { t } = useTranslation('models')
  return (
    <div className="grid gap-4">
      <Typography variant="body" as="p">
        {t('legacyLeaderboard.description')}
      </Typography>
      <div className="flex flex-wrap gap-3">
        <a href="/tests" className={buttonStyles({ variant: 'secondary' })}>
          {t('legacyLeaderboard.openTests')}
        </a>
        <a href="/tests/history" className={buttonStyles({ variant: 'ghost' })}>
          {t('legacyLeaderboard.openHistory')}
        </a>
      </div>
    </div>
  )
}
