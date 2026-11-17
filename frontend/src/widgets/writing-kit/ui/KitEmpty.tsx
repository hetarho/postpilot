import { useId, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { Typography } from '@/shared/ui'

/** Guide: an empty kit part, drawn as the slot the part will fill — the same dashed outline the
 *  home's graphic draws for a part that is not ready — with the part's own figure, what is missing
 *  and the one line saying what having it would do. Every kit screen's empty state is this one
 *  surface; each caller brings only its figure and its words. */
export function KitEmpty({
  figure,
  title,
  children,
  action,
  align = 'center',
  className,
}: {
  figure?: ReactNode
  title: ReactNode
  children?: ReactNode
  /** The one thing that fills the slot, when it is not the screen's own dock. */
  action?: ReactNode
  /** `start` for an empty slot inside a section rather than a whole screen's. */
  align?: 'center' | 'start'
  className?: string
}) {
  const id = useId()
  return (
    <section
      aria-labelledby={id}
      className={clsx(
        'border-content-tertiary/40 flex flex-col gap-3 rounded-2xl border-2 border-dashed',
        align === 'center' ? 'items-center px-6 py-10 text-center' : 'items-start px-5 py-6',
        className,
      )}
    >
      {figure && <span aria-hidden="true">{figure}</span>}
      <Typography variant={align === 'center' ? 'title' : 'fieldTitle'} as="h2" id={id}>
        {title}
      </Typography>
      {children && (
        <Typography
          variant="body"
          className={clsx(
            'text-content-secondary max-w-measure',
            align === 'center' && 'text-balance',
          )}
        >
          {children}
        </Typography>
      )}
      {action && <div className="mt-1 flex flex-wrap gap-3">{action}</div>}
    </section>
  )
}

/** The round accent badge an empty slot carries its part's glyph in. */
export function KitGlyph({ children }: { children: ReactNode }) {
  return (
    <span className="bg-badge-accent-bg text-badge-accent-fg flex size-14 items-center justify-center rounded-full [&>svg]:size-7">
      {children}
    </span>
  )
}
