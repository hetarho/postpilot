import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { useSession } from '@/entities/session'
import { Typography, typographyStyles } from '@/shared/ui'
import type { KitKey } from '../model/kit'
import { useWritingKit } from '../model/useWritingKit'
import { KitRing } from './KitRing'
import { useKitText } from './useKitText'

/** Guide: how every kit screen opens — what the thing IS and what it does to the owner's writing,
 *  in the screen's own large title and one sentence, with its illustration beside them. For a
 *  piece of the writing kit the illustration sits inside the very ring 홈 draws for it, only
 *  larger, and the line under the sentence says that ring's state and its one fact — so the
 *  screen and the picture that opened it say the same thing in the same shape.
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
  /** The picture of a screen outside the kit; a kit piece shows its own. */
  illustration?: string
  actions?: ReactNode
  /** `section` sits under a tab's own large title (영상's chips), so it takes the section's title
   *  role and a smaller illustration. */
  size?: 'page' | 'section'
  className?: string
}) {
  return (
    <header className={clsx('@container flex flex-col gap-5', className)}>
      {/* Side by side wherever the column has room for both, measured in rem so enlarged text
          moves the picture above the title instead of squeezing the title beside it. */}
      <div className="flex flex-col-reverse items-start gap-4 @xs:flex-row">
        <div className="flex w-full min-w-0 flex-1 flex-col gap-2">
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
        {kit ? (
          <KitPiece kit={kit} />
        ) : (
          illustration && (
            <img
              src={illustration}
              alt=""
              aria-hidden="true"
              className={clsx(
                'animate-rise shrink-0 object-contain drop-shadow-sm',
                size === 'page' ? 'size-24 sm:size-32' : 'size-16 sm:size-20',
              )}
            />
          )
        )}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}

function useKitElement(kit: KitKey) {
  const { user } = useSession()
  return useWritingKit(user?.id ?? '').elements[kit]
}

/** The piece as 홈 draws it, at the size of a screen's illustration. */
function KitPiece({ kit }: { kit: KitKey }) {
  const element = useKitElement(kit)
  return (
    <span className="animate-rise shrink-0">
      <KitRing kitKey={kit} state={element.state} fill={element.fill} size="lg" />
    </span>
  )
}

/** The piece's state in words, and its one fact. */
function KitStatus({ kit }: { kit: KitKey }) {
  const element = useKitElement(kit)
  const text = useKitText()
  return (
    <p className="mt-1 flex min-w-0 flex-wrap items-baseline gap-x-2" role="status">
      <span
        className={typographyStyles({
          variant: 'label',
          className: clsx(
            'shrink-0',
            element.state === 'ready' ? 'text-link-fg' : 'text-content-secondary',
          ),
        })}
      >
        {text.status(element)}
      </span>
      {element.state !== 'loading' &&
        element.state !== 'failed' &&
        element.fact.kind !== 'none' && (
          <span className={typographyStyles({ variant: 'meta', className: 'min-w-0 break-words' })}>
            {text.fact(element)}
          </span>
        )}
    </p>
  )
}
