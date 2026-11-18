import { useId, type ReactNode } from 'react'
import { MessagesSquare } from 'lucide-react'
import { Badge, Typography, cardStyles, typographyStyles } from '@/shared/ui'

/** A link the caller binds to its own route: the directory draws it, the caller knows where it
 *  goes. It must render one anchor carrying `className` and `children`. */
export type DirectoryLink = (props: { className: string; children: ReactNode }) => ReactNode

/** Guide: the unsaved conversations a template directory keeps apart from its saved tiles
 *  (TMPL-24, CLIP-42) — one compact row each, reopening its conversation. 템플릿 and 영상 템플릿
 *  draw the same rows; each binds only its own route and wording. */
export function ResumeConversations({
  heading,
  rows,
}: {
  heading: string
  rows: { key: string; label: string; link: DirectoryLink }[]
}) {
  const id = useId()
  if (rows.length === 0) return null
  return (
    <section aria-labelledby={id} className="mt-8">
      <Typography variant="eyebrow" as="h2" id={id}>
        {heading}
      </Typography>
      <ul className="mt-2 flex flex-col gap-2">
        {rows.map((row) => (
          <li key={row.key}>
            {row.link({
              className:
                'bg-badge-accent-bg hover:bg-card-bg-hover active:bg-card-bg-active flex min-h-14 items-center gap-3 rounded-2xl px-4 py-2',
              children: (
                <>
                  <MessagesSquare
                    aria-hidden="true"
                    className="text-badge-accent-fg size-5 shrink-0"
                  />
                  <span
                    className={typographyStyles({
                      variant: 'label',
                      className: 'text-content-primary min-w-0 flex-1 truncate',
                    })}
                  >
                    {row.label}
                  </span>
                </>
              ),
            })}
          </li>
        ))}
      </ul>
    </section>
  )
}

/** Guide: an empty directory is a dashed placeholder asking what the owner usually makes, with
 *  example kinds (THEME-29); it creates nothing — the screen's creation stands where it always
 *  does. */
export function TemplateDirectoryEmpty({ title, body }: { title: string; body: string }) {
  const id = useId()
  return (
    <section
      aria-labelledby={id}
      className="border-divider mt-8 flex flex-col items-start gap-3 rounded-3xl border border-dashed px-6 py-8"
    >
      <Typography variant="title" as="h2" id={id}>
        {title}
      </Typography>
      <Typography variant="body" as="p" className="text-content-secondary max-w-measure">
        {body}
      </Typography>
    </section>
  )
}

/** The saved tiles: one column on a phone, two from `sm`, three from `lg`. The region keeps a
 *  name for assistive technology without a visible heading over the grid. */
export function TemplateTileGrid({ label, children }: { label: string; children: ReactNode }) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="mt-8">
      <h2 id={id} className="sr-only">
        {label}
      </h2>
      <ul className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {children}
      </ul>
    </section>
  )
}

/** Guide: one saved template as a tile — its miniature in a well, clipped and faded where the
 *  tile ends, then its name, a line saying what it holds, its use count and at most one work
 *  state. The whole tile is the one link; deleting lives on the template's own screen, never
 *  here. The tiles stand in place as the list answers, in the grid the loading cards held. */
export function TemplateTile({
  link,
  miniature,
  name,
  description,
  count,
  workState,
}: {
  link: DirectoryLink
  /** What the template makes, drawn small and static — decorative beside the name. */
  miniature: ReactNode
  name: string
  description?: ReactNode
  /** How many works use it, as operational metadata. */
  count: string
  /** At most one work state, already worded (see `templateWorkState`). */
  workState?: string
}) {
  return (
    <li>
      {link({
        className: cardStyles({
          interactive: true,
          padding: 'none',
          className: 'group flex h-full flex-col overflow-hidden',
        }),
        children: (
          <>
            <div
              aria-hidden="true"
              className="bg-card-well relative h-52 overflow-hidden px-5 pt-5"
            >
              {miniature}
              <div className="from-card-well pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-linear-to-t to-transparent" />
            </div>
            <div className="flex flex-1 flex-col gap-1 p-5">
              <span
                className={typographyStyles({
                  variant: 'fieldTitle',
                  className: 'text-content-primary line-clamp-2 break-words',
                })}
              >
                {name}
              </span>
              {description && (
                <span
                  className={typographyStyles({
                    variant: 'body',
                    className: 'text-content-secondary line-clamp-2',
                  })}
                >
                  {description}
                </span>
              )}
              <span className="mt-auto flex flex-wrap items-center gap-2 pt-2">
                <Badge tone="neutral">{count}</Badge>
                {workState && (
                  <span
                    role="status"
                    className={typographyStyles({ variant: 'meta', className: 'min-w-0' })}
                  >
                    {workState}
                  </span>
                )}
              </span>
            </div>
          </>
        ),
      })}
    </li>
  )
}
