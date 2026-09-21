import { useTranslation } from 'react-i18next'
import { Typography } from '@/shared/ui'

export function ModelPageHeader({
  title,
  description,
}: {
  title: 'modelSettings' | 'comparison' | 'history' | 'leaderboardTitle'
  description:
    | 'settingsDescription'
    | 'comparisonDescription'
    | 'historyDescription'
    | 'leaderboardDescription'
}) {
  const { t } = useTranslation('models')
  return (
    <header>
      <Typography variant="display" className="sr-only lg:not-sr-only">
        {t(`page.${title}`)}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t(`page.${description}`)}
      </Typography>
    </header>
  )
}
