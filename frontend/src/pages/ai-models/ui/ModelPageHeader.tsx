import { useTranslation } from 'react-i18next'
import { TitleRow, Typography } from '@/shared/ui'

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
      <TitleRow variant="display">
        <Typography variant="display">{t(`page.${title}`)}</Typography>
      </TitleRow>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t(`page.${description}`)}
      </Typography>
    </header>
  )
}
