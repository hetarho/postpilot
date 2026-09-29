import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { SegmentedControl } from '@/shared/ui'
import type { ModelLabTabName } from '@/entities/model-experiment'
import { stageOfTab, useModelStage } from '../model/useModelStage'

const STAGE_TABS: readonly ModelLabTabName[] = ['observe', 'write']
const LAB_TABS: readonly ModelLabTabName[] = ['observe', 'write', 'voice']

export function ModelStageTabs({
  to,
  withVoice = false,
}: {
  to: '/ai-models/compare' | '/ai-models/experiments' | '/ai-models/leaderboard'
  /** 말투 반영, on the compare and history pages; the leaderboard keeps the two stages. */
  withVoice?: boolean
}) {
  const { t } = useTranslation('models')
  const { stage: tab } = useModelStage()
  const stage = withVoice ? tab : stageOfTab(tab)
  const navigate = useNavigate()
  return (
    <div className="bg-surface-base top-chrome sticky z-10 -mx-4 mt-4 px-4 py-2 sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
      <SegmentedControl
        value={stage}
        // The stages the lab compares (MODEL-30): analyze keeps its active selection on 모델
        // 변경 and has no comparison, history or board of its own.
        options={(withVoice ? LAB_TABS : STAGE_TABS).map((value) => ({
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
