import type { GenerationJob } from '../model/types'
import { progressLabel } from '../model/types'

export function ProgressLine({ job }: { job: GenerationJob }) {
  return (
    <p role="status" className="bg-notice-info-bg text-notice-info-fg rounded-md px-3 py-2 text-sm">
      {progressLabel(job)}
    </p>
  )
}
