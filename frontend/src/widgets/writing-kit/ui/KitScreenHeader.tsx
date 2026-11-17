import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { useSession } from '@/entities/session'
import { Typography, typographyStyles } from '@/shared/ui'
import type { KitKey } from '../model/kit'
import { useWritingKit } from '../model/useWritingKit'
import { KitRing } from './KitRing'
import { useKitText } from './useKitText'

/** Guide: how every kit screen opens — what the thing IS and what it does to the owner's writing,
 *  in the screen's own large title and one sentence, its illustration beside them, and (for a
 *  piece of the writing kit) the same ring 홈 draws for it, with its state and its one fact. So
 *  the screen and the picture that opened it say the same thing in the same shape.
 *
 *  `actions` is the screen's one heading row of actions; nothing else goes here. */
export function KitScreenHeader({
  kit,
  title,
  statement,
  illustration,
  actions,
  size = 'page',
  className,
}: {
  /** The kit piece this screen makes ready; none for a screen outside the writing kit. */
  kit?: KitKey
  title: string
  statement: ReactNode
  illustration: string
  actions?: ReactNode
  /** `section` sits under a tab's own large title (영상's chips), so it takes the section's title
   *  role and a smaller illustration. */
  size?: 'page' | 'section'
  className?: string
}) {
  return (
    <header className={clsx('flex flex-col gap-5', className)}>
      <div className="flex items-start gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <Typography
            variant={size === 'page' ? 'largeTitle' : 'title'}
            as="h1"
            className="break-words"
          >
            {title}
          </Typography>
          <Typography variant="body" className="text-content-secondary max-w-measure text-pretty">
            {statement}
          </Typography>
          {kit && <KitStatus kit={kit} />}
        </div>
        <img
          src={illustration}
          alt=""
          aria-hidden="true"
          className={clsx(
            'animate-rise shrink-0 object-contain drop-shadow-sm',
            size === 'page' ? 'size-24 sm:size-32' : 'size-16 sm:size-20',
          )}
        />
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}

/** The piece's ring as 홈 draws it, its state in words and its one fact. */
function KitStatus({ kit }: { kit: KitKey }) {
  const { user } = useSession()
  const { elements } = useWritingKit(user?.id ?? '')
  const text = useKitText()
  const element = elements[kit]
  return (
    <p className="mt-1 flex min-w-0 items-center gap-2" role="status">
      <KitRing kitKey={kit} state={element.state} fill={element.fill} size="sm" />
      <span
        className={typographyStyles({
          variant: 'label',
          className: clsx(
            'shrink-0',
            element.state === 'ready' ? 'text-link-fg' : 'text-content-secondary',
          ),
        })}
      >
        {text.state(element)}
      </span>
      {element.state !== 'loading' && element.fact.kind !== 'none' && (
        <span className={typographyStyles({ variant: 'meta', className: 'min-w-0 truncate' })}>
          {text.fact(element)}
        </span>
      )}
    </p>
  )
}
