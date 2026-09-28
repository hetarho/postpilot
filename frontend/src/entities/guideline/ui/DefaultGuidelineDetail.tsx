import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Typography } from '@/shared/ui'
import type { DefaultGuideline } from '../model/types'

/** What a 기본 지침 says (GUIDE-47, GUIDE-48): its text word for word — it is exactly what the
 *  writer is given — and the note on which runs it reaches. Shared by an open row of the list and a
 *  row of the 기본 지침 sheet; each caller supplies its own action. */
export function DefaultGuidelineDetail({
  guideline,
  action,
}: {
  guideline: Pick<DefaultGuideline, 'text' | 'koreanTargetOnly' | 'memoriesOnly'>
  /** `적용 안함` on an open row, `추가` or `적용 중` in the sheet. */
  action?: ReactNode
}) {
  const { t } = useTranslation('guidelines')
  return (
    <div>
      <Typography variant="body" className="text-content-secondary whitespace-pre-wrap">
        {guideline.text}
      </Typography>
      {guideline.koreanTargetOnly && (
        <Typography variant="meta" as="p" className="mt-1">
          {t('defaults.koreanOnly')}
        </Typography>
      )}
      {guideline.memoriesOnly && (
        <Typography variant="meta" as="p" className="mt-1">
          {t('defaults.memoriesOnly')}
        </Typography>
      )}
      {action && <div className="mt-2 flex flex-wrap items-center gap-2">{action}</div>}
    </div>
  )
}
