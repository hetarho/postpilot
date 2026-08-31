import { useTranslation } from 'react-i18next'
import { Badge, Notice } from '@/shared/ui'
import type { GenerationJob } from '../model/types'
import { progressLabel } from '../model/types'

export function ProgressLine({ job }: { job: GenerationJob }) {
  const { t } = useTranslation(['posts', 'common'])
  return (
    <Notice tone="info" role="status">
      {progressLabel(job)}
      {job.targetLanguage && (
        <Badge>{t(`contentLanguage.${job.targetLanguage}`, { ns: 'common' })}</Badge>
      )}
    </Notice>
  )
}
