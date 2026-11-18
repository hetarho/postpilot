import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { TitleRow, Typography } from '@/shared/ui'
import type { KitKey } from '../model/kit'
import { KitRing } from './KitRing'
import { useKitElement } from './useKitElement'

/** Guide: how every kit screen opens — what the thing IS and what it does to the owner's writing,
 *  in the screen's own large title and one sentence, with its illustration beside them. For a
 *  piece of the writing kit the illustration sits inside the very ring 홈 draws for it, only
 *  larger, so the screen and the picture that opened it are the same shape. The piece's state
 *  in words stays on 홈, under the piece; here the ring says it and the screen shows the rest.
 *
 *  The header is the screen's frame, so it is there from the first frame and does not rise in:
 *  only the ring's own stroke draws.
 *
 *  `actions` is the screen's one heading row of actions; nothing else goes here. The screen's way
 *  back, when it has one, stands at the start of the title's row rather than on a row above it. */
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
          <TitleRow variant={size === 'page' ? 'largeTitle' : 'title'}>
            <Typography
              variant={size === 'page' ? 'largeTitle' : 'title'}
              as="h1"
              className="break-words"
            >
              {title}
            </Typography>
          </TitleRow>
          <Typography variant="body" className="text-content-secondary max-w-measure text-pretty">
            {statement}
          </Typography>
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
                'shrink-0 object-contain drop-shadow-sm',
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

/** The piece as 홈 draws it, at the size of a screen's illustration. */
function KitPiece({ kit }: { kit: KitKey }) {
  const element = useKitElement(kit)
  return (
    <span className="shrink-0">
      <KitRing kitKey={kit} state={element.state} fill={element.fill} size="lg" />
    </span>
  )
}
