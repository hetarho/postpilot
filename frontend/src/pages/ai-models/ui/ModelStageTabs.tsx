import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { SegmentedControl } from '@/shared/ui'
import { useModelStage } from '../model/useModelStage'

export function ModelStageTabs({
  to,
}: {
  to: '/ai-models/compare' | '/ai-models/experiments' | '/ai-models/leaderboard'
}) {
  const { t } = useTranslation('models')
  const { stage } = useModelStage()
  const navigate = useNavigate()
  return (
    <div className="bg-surface-base top-chrome sticky z-10 -mx-4 mt-4 px-4 py-2 sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
      <SegmentedControl
        value={stage}
        options={(['observe', 'analyze', 'write'] as const).map((value) => ({
          value,
          label: t(`stage.${value}`),
        }))}
        // The other filters ride along: changing the stage on the leaderboard must not throw
        // away the window and scope the reader chose (MODEL-44).
        onChange={(stage) => void navigate({ to, search: (previous) => ({ ...previous, stage }) })}
        ariaLabel={t('page.stageAria')}
      />
    </div>
  )
}
