import { skipReasonLabel } from '../model/filter'
import type { UploadItem } from '../model/upload-batch'
import { Button } from '@/shared/ui'

/** The files that were never uploaded, each with why (PRD F-2: "건너뜀"). */
export function SkippedList({
  items,
  onDismiss,
}: {
  items: readonly UploadItem[]
  onDismiss: (id: string) => void
}) {
  const skipped = items.filter((item) => item.status === 'skipped')
  if (skipped.length === 0) return null

  return (
    <section aria-labelledby="skipped-heading" className="text-content-secondary text-xs">
      <h3 id="skipped-heading" className="text-content-secondary font-medium">
        건너뜀
      </h3>
      <ul className="mt-1 flex flex-col gap-1">
        {skipped.map((item) => (
          <li key={item.id} className="flex min-h-11 items-center gap-2">
            <span className="min-w-0 flex-1 truncate">{item.name}</span>
            <span className="text-content-tertiary shrink-0">
              — {item.reason && skipReasonLabel(item.reason)}
            </span>
            <Button
              variant="danger"
              size="icon"
              onClick={() => onDismiss(item.id)}
              aria-label={`${item.name} 목록에서 지우기`}
            >
              <span aria-hidden="true">×</span>
            </Button>
          </li>
        ))}
      </ul>
    </section>
  )
}
