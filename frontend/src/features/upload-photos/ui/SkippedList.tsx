import { skipReasonLabel } from '../model/filter'
import type { UploadItem } from '../model/upload-batch'

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
    <section aria-labelledby="skipped-heading" className="text-xs text-text-muted">
      <h3 id="skipped-heading" className="font-medium text-text-muted">
        건너뜀
      </h3>
      <ul className="mt-1 flex flex-col gap-1">
        {skipped.map((item) => (
          <li key={item.id} className="flex items-center gap-2">
            <span className="truncate">{item.name}</span>
            <span className="text-text-subtle">— {item.reason && skipReasonLabel(item.reason)}</span>
            <button
              type="button"
              onClick={() => onDismiss(item.id)}
              aria-label={`${item.name} 목록에서 지우기`}
              className="text-text-faint hover:text-text"
            >
              ✕
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}
