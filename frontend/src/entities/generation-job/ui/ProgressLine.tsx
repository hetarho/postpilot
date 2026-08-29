import { Notice } from '@/shared/ui'
import type { GenerationJob } from '../model/types'
import { progressLabel } from '../model/types'

export function ProgressLine({ job }: { job: GenerationJob }) {
  return (
    <Notice tone="info" role="status">
      {progressLabel(job)}
    </Notice>
  )
}
