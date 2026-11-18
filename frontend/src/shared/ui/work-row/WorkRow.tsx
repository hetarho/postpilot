import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { ChevronRight } from 'lucide-react'
import { typographyStyles } from '../typography/typographyStyles'

/** What the caller's link element is handed: the stretched-target class and the row's content. */
export type WorkRowLinkProps = { className: string; children: ReactNode }

/** Guide: one item of a work list as ONE target (THEME-83). The whole row opens the item through
 *  the caller's link, stretched over the row; the title keeps one truncated line and the meta
 *  line one more under it, so a phone holds about twice the items the earlier cards did. A
 *  trailing action exists only for somewhere other than the item (a result, a published page, a
 *  file); it stands above the stretched link as its own target, never nested inside it.
 *
 *  The primitive knows no router and no domain: the caller supplies the link element through
 *  `link`, every word and the meta line's content. Rows are `li`s for a `WorkRowList`. */
export function WorkRow({
  link,
  title,
  meta,
  leading,
  tint = false,
  cue,
  action,
  className,
  ...data
}: {
  /** Renders the item's own link with the given class and children (`<Link className>`). */
  link: (props: WorkRowLinkProps) => ReactNode
  title: ReactNode
  /** The one line under the title: a state badge, then what the item was made with and when. */
  meta?: ReactNode
  /** An optional glyph before the text, decorative. */
  leading?: ReactNode
  /** The accent-subtle tint of the one row the screen leads with (THEME-18). */
  tint?: boolean
  /** A short word before the chevron naming what opening the row does (`이어서`). */
  cue?: ReactNode
  /** At most one action opening somewhere other than the item. */
  action?: ReactNode
  className?: string
} & { [key: `data-${string}`]: string | undefined }) {
  return (
    <li
      {...data}
      className={clsx(
        'relative flex min-h-16 min-w-0 items-center gap-2 px-4 py-2 transition-colors',
        tint ? 'bg-badge-accent-bg' : 'hover:bg-card-bg-hover active:bg-card-bg-active',
        className,
      )}
    >
      {link({
        className:
          'flex min-h-11 min-w-0 flex-1 items-center gap-3 py-1 after:absolute after:inset-0',
        children: (
          <>
            {leading && (
              <span aria-hidden="true" className="flex shrink-0">
                {leading}
              </span>
            )}
            <span className="flex min-w-0 flex-1 flex-col gap-1">
              <span
                className={typographyStyles({
                  variant: 'fieldTitle',
                  className: 'text-content-primary truncate',
                })}
              >
                {title}
              </span>
              {meta && (
                <span
                  className={typographyStyles({
                    variant: 'meta',
                    className: 'flex min-w-0 items-center gap-2 overflow-hidden whitespace-nowrap',
                  })}
                >
                  {meta}
                </span>
              )}
            </span>
            {cue && (
              <span
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-link-fg-current shrink-0',
                })}
              >
                {cue}
              </span>
            )}
            <ChevronRight
              aria-hidden="true"
              className={clsx(
                'size-5 shrink-0',
                cue ? 'text-link-fg-current' : 'text-content-tertiary',
              )}
            />
          </>
        ),
      })}
      {action && <div className="relative z-10 flex shrink-0 items-center">{action}</div>}
    </li>
  )
}

/** The rows of one work group on one card, a hairline apart (THEME-13 / THEME-83). */
export function WorkRowList({
  label,
  children,
  className,
}: {
  label?: string
  children: ReactNode
  className?: string
}) {
  return (
    <ul
      aria-label={label}
      className={clsx(
        'bg-card-bg divide-divider min-w-0 divide-y overflow-hidden rounded-xl shadow-sm',
        className,
      )}
    >
      {children}
    </ul>
  )
}
