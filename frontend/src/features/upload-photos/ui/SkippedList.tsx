import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('posts')
  const skipped = items.filter((item) => item.status === 'skipped')
  if (skipped.length === 0) return null

  return (
    <section aria-labelledby="skipped-heading" className="text-content-secondary text-sm">
      <h3 id="skipped-heading" className="text-content-secondary font-medium">
        {t('upload.skipped')}
      </h3>
      {/* A hairline between rows instead of a 4px gap: the rows have no background of their own
          (§1.3), and 4px put two 44px dismiss boxes under the same thumb (§4.1). */}
      <ul className="divide-divider mt-1 flex flex-col divide-y">
        {skipped.map((item) => (
          <li key={item.id} className="flex min-h-11 flex-wrap items-center gap-x-2 gap-y-1 py-2">
            <span className="min-w-0 flex-1 truncate">{item.name}</span>
            <Button
              variant="danger"
              size="icon"
              onClick={() => onDismiss(item.id)}
              aria-label={t('upload.dismissAria', { name: item.name })}
              className="text-xl"
            >
              <span aria-hidden="true">×</span>
            </Button>
            {/* The reason takes its own line. `shrink-0` on a flex item pins it at its
                max-content width, and the extension reason is one unbroken 425px line at 360px:
                it pushed the whole document — title, memo, generate button — into horizontal
                scroll and crushed the filename beside it to zero (§8.5). */}
            <span className="text-content-tertiary w-full">
              {item.reason && skipReasonLabel(item.reason)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}
