import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Search, X } from 'lucide-react'
import { Button, Sheet, Typography, WorkRowList, typographyStyles } from '@/shared/ui'

/** Guide: one status group of a work list — 작성 중 or 완성 — a heading with its count over its
 *  work rows on one card (THEME-83). */
export function WorkGroup({
  heading,
  count,
  listLabel,
  children,
  footer,
}: {
  heading: string
  count: number
  /** The list's own accessible name, when it differs from the heading. */
  listLabel?: string
  children: ReactNode
  footer?: ReactNode
}) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="min-w-0">
      <h2 id={id} className="mb-3 flex items-baseline gap-2">
        <span className={typographyStyles({ variant: 'title' })}>{heading}</span>
        <span
          className={typographyStyles({ variant: 'label', className: 'text-content-tertiary' })}
        >
          {count}
        </span>
      </h2>
      <WorkRowList label={listLabel}>{children}</WorkRowList>
      {footer}
    </section>
  )
}

/** Guide: search and filters live behind ONE 찾기 button that opens a bottom sheet, so the list
 *  opens on its work rather than on a form. While a narrowing is on, the button says how many
 *  parts are active and a ✕ beside it clears them in one press; it stays a secondary button, since
 *  the screen's one filled CTA is its creation button (THEME-18). */
export function FindSheet({
  active,
  onClear,
  children,
}: {
  /** How many narrowings are on (a query, a status). */
  active: number
  onClear: () => void
  children: ReactNode
}) {
  const { t } = useTranslation('channel')
  const [open, setOpen] = useState(false)
  const headingId = useId()
  return (
    <div className="flex items-center gap-2">
      <Button
        variant="secondary"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        className="shadow-sm"
      >
        <Search aria-hidden="true" className="size-5" />
        {active ? t('work.findActive', { count: active }) : t('work.find')}
      </Button>
      {active > 0 && (
        <Button variant="ghost" size="icon" aria-label={t('work.clearFind')} onClick={onClear}>
          <X aria-hidden="true" className="size-5" />
        </Button>
      )}
      <Sheet
        open={open}
        labelledBy={headingId}
        onClose={() => setOpen(false)}
        header={
          <Typography variant="title" as="h2" id={headingId} className="px-5 pt-5 sm:px-6">
            {t('work.findTitle')}
          </Typography>
        }
        bodyClassName="px-5 pb-4 sm:px-6"
        footer={
          <div className="px-5 pt-2 pb-5 sm:px-6">
            <Button variant="cta" size="hero" className="w-full" onClick={() => setOpen(false)}>
              {t('work.findDone')}
            </Button>
          </div>
        }
      >
        {children}
      </Sheet>
    </div>
  )
}
