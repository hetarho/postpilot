import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { SurfaceButton, typographyStyles } from '@/shared/ui'
import type { KitKey, KitState } from '../model/kit'
import { KitRing } from './KitRing'

const ROW_STYLES =
  'bg-card-bg hover:bg-card-bg-hover active:bg-card-bg-active flex w-full min-w-0 items-center gap-3 rounded-2xl py-2 pr-4 pl-2 text-left shadow-sm'

interface KitStepRowProps {
  kitKey: KitKey
  state: KitState
  fill: number
  /** The object drawn in the ring when it is not the kit piece's own (→KitRing). */
  illustration?: string
  /** A small line over the action words — 홈's `다음`. */
  label?: string
  /** The action words: what pressing the row makes. */
  children: ReactNode
  /** Where the row goes when it is a link… */
  link?: { to: string; search?: Record<string, unknown>; hash?: string }
  /** …or what it does when it is a button. */
  onClick?: () => void
}

/** One missing piece of the writing kit and the step that makes it: the piece's small ring, the
 *  action words and an arrow, the whole row one target. 홈's next step (THEME-79) and a new post's
 *  or clip's offer (POST-121, CLIP-223) are this one row — a link where it opens a screen, a
 *  button where its caller acts. */
export function KitStepRow({
  kitKey,
  state,
  fill,
  illustration,
  label,
  children,
  link,
  onClick,
}: KitStepRowProps) {
  const content = (
    <>
      <KitRing kitKey={kitKey} state={state} fill={fill} illustration={illustration} size="sm" />
      <span className="flex min-w-0 flex-1 flex-col">
        {label && <span className={typographyStyles({ variant: 'meta' })}>{label}</span>}
        <span
          className={typographyStyles({
            variant: 'label',
            className: 'text-content-primary text-pretty break-keep',
          })}
        >
          {children}
        </span>
      </span>
      <ArrowRight aria-hidden="true" className="text-link-fg size-5 shrink-0" />
    </>
  )
  if (link)
    return (
      <Link to={link.to} search={link.search} hash={link.hash} className={ROW_STYLES}>
        {content}
      </Link>
    )
  return (
    <SurfaceButton presentation="content" className={ROW_STYLES} onClick={onClick}>
      {content}
    </SurfaceButton>
  )
}
