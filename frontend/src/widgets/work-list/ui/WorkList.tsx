import { useId, useState, type ComponentType, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ArrowRight, Search, X } from 'lucide-react'
import {
  Badge,
  Button,
  Sheet,
  Typography,
  buttonStyles,
  cardStyles,
  typographyStyles,
  type BadgeTone,
} from '@/shared/ui'

const TILE_TONE: Record<BadgeTone, string> = {
  neutral: 'bg-badge-neutral-bg text-badge-neutral-fg',
  accent: 'bg-badge-accent-bg text-badge-accent-fg',
  success: 'bg-notice-success-bg text-notice-success-fg',
  done: 'bg-badge-done-bg text-badge-done-fg',
  warning: 'bg-notice-warning-bg text-notice-warning-fg',
  info: 'bg-notice-info-bg text-notice-info-fg',
  danger: 'bg-notice-danger-bg text-notice-danger-fg',
}

/** Guide: a work card's leading tile — a soft square in the status chip's own tone with a glyph
 *  for what the work is, so 내 글 is scanned by shape and colour before a word is read. */
export function StatusTile({
  tone,
  icon: Icon,
  size = 'md',
}: {
  tone: BadgeTone
  icon: ComponentType<{ className?: string }>
  size?: 'md' | 'lg'
}) {
  return (
    <span
      aria-hidden="true"
      className={clsx(
        'flex shrink-0 items-center justify-center rounded-lg',
        size === 'lg' ? 'size-16' : 'size-12 sm:size-14',
        TILE_TONE[tone],
      )}
    >
      <Icon className={size === 'lg' ? 'size-8' : 'size-6'} />
    </span>
  )
}

/** Guide: 이어서 쓰기 — the latest unfinished work as the one big card a list opens on, so a busy
 *  owner coming back taps once. The whole card is the link (`renderLink`), its 계속하기 a picture
 *  of the press rather than a second target — and never a second filled CTA: the screen's one is
 *  its creation button, and the card's tint already says it comes first (THEME-18). */
export function ContinueHero({
  kind,
  title,
  status,
  icon,
  meta,
  renderLink,
}: {
  /** Which channel the work is — 블로그, 네이버 클립 … */
  kind: string
  title: string
  status: { label: string; tone: BadgeTone }
  icon: ComponentType<{ className?: string }>
  meta?: ReactNode
  renderLink: (props: { className: string; children: ReactNode }) => ReactNode
}) {
  const { t } = useTranslation('channel')
  const id = useId()
  return (
    <section aria-labelledby={id} className="min-w-0">
      <Typography variant="title" as="h2" id={id} className="mb-3">
        {t('work.continue')}
      </Typography>
      {renderLink({
        className: cardStyles({
          interactive: true,
          padding: 'lg',
          // The one card on the screen in the accent's soft tint: it is the screen's first answer.
          className:
            'bg-badge-accent-bg hover:bg-badge-accent-bg active:bg-badge-accent-bg relative flex flex-col gap-5 overflow-hidden',
        }),
        children: (
          <>
            <span className="flex min-w-0 items-start gap-4">
              <StatusTile tone={status.tone} icon={icon} size="lg" />
              <span className="flex min-w-0 flex-1 flex-col gap-2">
                <span className="flex flex-wrap items-center gap-2">
                  <span className={typographyStyles({ variant: 'label' })}>{kind}</span>
                  <Badge tone={status.tone}>{status.label}</Badge>
                </span>
                <Typography variant="title" as="span" className="line-clamp-2">
                  {title}
                </Typography>
                {meta && <span className={typographyStyles({ variant: 'meta' })}>{meta}</span>}
              </span>
            </span>
            <span
              aria-hidden="true"
              className={buttonStyles({
                variant: 'secondary',
                className: 'pointer-events-none w-full',
              })}
            >
              {t('work.continueAction')}
              <ArrowRight className="size-5" />
            </span>
          </>
        ),
      })}
    </section>
  )
}

/** Guide: 이어서 쓰기 as a list's first section — the list's own card for its newest unfinished
 *  work, rendered large (`heroCardStyles`) under the heading, so the card keeps every link and
 *  label it has in the list and the owner's next tap is the 계속하기 on it. */
export function ContinueSection({ children }: { children: ReactNode }) {
  const { t } = useTranslation('channel')
  const id = useId()
  return (
    <section aria-labelledby={id} className="min-w-0">
      <Typography variant="title" as="h2" id={id} className="mb-3">
        {t('work.continue')}
      </Typography>
      <ul>{children}</ul>
    </section>
  )
}

/** Guide: one status group of a work list — 작성 중 or 완성 — a heading with its count over a
 *  column of cards (two on the desk). */
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
      <ul aria-label={listLabel} className="grid gap-3 lg:grid-cols-2 lg:gap-4">
        {children}
      </ul>
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
