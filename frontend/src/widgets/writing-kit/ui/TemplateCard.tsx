import { useId, type ReactNode } from 'react'
import { MessagesSquare } from 'lucide-react'
import { cardStyles, typographyStyles } from '@/shared/ui'

/** Guide: one saved template, drawn as the shape of what it makes — its name, then its outline
 *  (a post's headings and asks, a video's frame and stages), then how much it is used and any
 *  work still open on it. The whole card is the way into the template, where it is changed by
 *  talking and deleted; the list itself has nothing else to press. 글 템플릿 and 영상 템플릿 are
 *  this one card, each list binding only its own data and its own link. */
export function TemplateCard({
  index,
  name,
  description,
  outline,
  meta,
  status,
  renderLink,
}: {
  /** Its place in the list, for the arrival's stagger. */
  index: number
  name: string
  description?: ReactNode
  outline?: ReactNode
  meta?: ReactNode
  /** Work still open on the template, in one line. */
  status?: ReactNode
  renderLink: (link: {
    className: string
    'aria-labelledby': string
    children: ReactNode
  }) => ReactNode
}) {
  const nameId = useId()
  return (
    <li className="animate-rise" style={{ animationDelay: `${index * 50}ms` }}>
      {renderLink({
        'aria-labelledby': nameId,
        className: cardStyles({
          interactive: true,
          className: 'flex h-full min-w-0 flex-col gap-3',
        }),
        children: (
          <>
            <span className="flex min-w-0 flex-col gap-1">
              <span
                id={nameId}
                className={typographyStyles({
                  variant: 'fieldTitle',
                  className: 'text-content-primary line-clamp-2 break-words',
                })}
              >
                {name}
              </span>
              {description && (
                <span className={typographyStyles({ variant: 'meta', className: 'line-clamp-2' })}>
                  {description}
                </span>
              )}
            </span>
            {outline}
            {(meta || status) && (
              <span className="mt-auto flex flex-wrap items-center gap-2">
                {meta}
                {status && (
                  <span
                    role="status"
                    className={typographyStyles({ variant: 'meta', className: 'text-link-fg' })}
                  >
                    {status}
                  </span>
                )}
              </span>
            )}
          </>
        ),
      })}
    </li>
  )
}

/** A template conversation not saved yet: what it is called, where it stands, and the way back
 *  into it. Both template lists show unsaved work as this card. */
export function UnsavedTemplateCard({
  title,
  status,
  renderLink,
}: {
  title: string
  status: string
  renderLink: (link: { className: string; children: ReactNode }) => ReactNode
}) {
  return (
    <li>
      {renderLink({
        className: cardStyles({
          interactive: true,
          className: 'flex min-w-0 items-center gap-3',
        }),
        children: (
          <>
            <span
              aria-hidden="true"
              className="bg-badge-accent-bg text-badge-accent-fg flex size-11 shrink-0 items-center justify-center rounded-full"
            >
              <MessagesSquare className="size-5" />
            </span>
            <span className="flex min-w-0 flex-1 flex-col">
              <span
                className={typographyStyles({
                  variant: 'fieldTitle',
                  className: 'text-content-primary truncate',
                })}
              >
                {title}
              </span>
              <span className={typographyStyles({ variant: 'meta' })}>{status}</span>
            </span>
          </>
        ),
      })}
    </li>
  )
}
