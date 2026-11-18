import { useId, type CSSProperties } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ChevronRight } from 'lucide-react'
import { typographyStyles } from '@/shared/ui'
import { KIT_HASH, hasKitDefault, type KitElement, type KitKey } from '../model/kit'
import { CreationCentre } from './CreationCentre'
import { KitDefaultControl } from './KitDefaultControl'
import { KitRing } from './KitRing'
import { useKitText } from './useKitText'

/** Where each piece sits on the orbit, in percent of the square: a pentagon on a circle of 36%
 *  radius around the pen, clockwise from the top (the order of `KIT_KEYS`). */
const ORBIT: Record<KitKey, { x: number; y: number }> = {
  voice: { x: 50, y: 18 },
  template: { x: 84.24, y: 38.88 },
  guideline: { x: 71.16, y: 79.12 },
  ai: { x: 28.84, y: 79.12 },
  memory: { x: 15.76, y: 38.88 },
}

/** Guide: the writing kit as one picture — the pen in the middle, the five things that feed it
 *  on an orbit around it, each wired to the pen by a spoke. A ready piece is a full ring on a
 *  solid spoke; a piece being made draws as much ring as its own meter; a missing one is a dashed
 *  outline on a dashed spoke. The halo behind the pen glows with how much of the kit is there,
 *  and only once all of it is does it breathe — motion lives in the halo, never in a stroke.
 *
 *  Every element works one way: its icon goes, its name chooses. The centre's icon starts the
 *  page's 글쓰기 or 영상 만들기 and its name switches between them (`CreationCentre`); a piece's
 *  ring opens the screen that makes it ready, and the name of a piece with a default is a dropdown
 *  that changes that default in place (`KitDefaultControl`).
 *
 *  One DOM in two layouts: a circle wherever the container has room for one (≥ 20rem, measured
 *  in rem so enlarged text falls back on its own), and otherwise a column — the centre and then
 *  the pieces as rows — where nothing can overlap. */
export function WritingKit({
  elements,
  fill,
  complete,
  className,
}: {
  elements: readonly KitElement[]
  /** The rings' fills, averaged: how strongly the halo glows. */
  fill: number
  complete: boolean
  className?: string
}) {
  const { t } = useTranslation('creation')
  return (
    <div className={clsx('@container', className)}>
      <div className="relative flex flex-col gap-4 @xs:mb-6 @xs:block @xs:aspect-square @xs:w-full">
        {/* The orbit and the spokes. Percent coordinates, a stroke that never scales. */}
        <svg
          aria-hidden="true"
          viewBox="0 0 100 100"
          preserveAspectRatio="none"
          className="absolute inset-0 hidden size-full @xs:block"
        >
          <circle
            cx="50"
            cy="50"
            r="36"
            fill="none"
            strokeWidth="1"
            strokeDasharray="2 6"
            vectorEffect="non-scaling-stroke"
            className="stroke-divider"
          />
          {elements.map((element) => {
            const point = ORBIT[element.key]
            const solid = element.state === 'ready' || element.state === 'partial'
            return (
              <line
                key={element.key}
                x1="50"
                y1="50"
                x2={point.x}
                y2={point.y}
                strokeWidth={solid ? 2 : 1.5}
                strokeLinecap="round"
                strokeDasharray={solid ? undefined : '3 6'}
                vectorEffect="non-scaling-stroke"
                className={clsx(
                  'duration-slow transition-colors',
                  element.state === 'ready'
                    ? 'stroke-meter-fill'
                    : element.state === 'partial'
                      ? 'stroke-progress-step-todo'
                      : 'stroke-content-tertiary',
                )}
              />
            )
          })}
        </svg>

        {/* The centre: the screen's one way to start, in a halo that grows with the kit. */}
        <CreationCentre complete={complete} fill={fill} />

        <ol
          aria-label={t('kit.aria')}
          className="grid grid-cols-[minmax(0,1fr)] gap-1 @xs:pointer-events-none @xs:absolute @xs:inset-0 @xs:block"
        >
          {elements.map((element) => {
            const point = ORBIT[element.key]
            return (
              // No entrance of its own: the pieces stand where they belong from the first frame,
              // each ring holding its place while its data arrives, so nothing rises or staggers
              // in as a list answers.
              <li
                key={element.key}
                style={{ '--kit-x': `${point.x}%`, '--kit-y': `${point.y}%` } as CSSProperties}
                className={clsx(
                  '@xs:pointer-events-auto @xs:absolute @xs:top-(--kit-y) @xs:left-(--kit-x) @xs:-translate-x-1/2',
                  // Anchor the RING's centre on the orbit point: its words hang below it, or above
                  // it for the top piece. Half the ring: 2rem, and 2.5rem once it grows. The top
                  // piece stands alone on the centre line, so its words take a wider line.
                  point.y < 30
                    ? '@xs:w-36 @xs:-translate-y-[calc(100%-2rem)] @md:w-44 @md:-translate-y-[calc(100%-2.5rem)]'
                    : '@xs:w-28 @xs:-translate-y-8 @md:w-36 @md:-translate-y-10',
                )}
              >
                <KitPiece element={element} top={point.y < 30} />
              </li>
            )
          })}
        </ol>
      </div>
    </div>
  )
}

/** One piece on the orbit, or one row of the column: its ring, its name and its one fact. The
 *  ring is the way to the piece's screen; the name of a piece with a default is the dropdown that
 *  changes it (`KitDefaultControl`). 기억 has no default (MEM-29), so its name is the way to its
 *  screen too, the link stretched over the whole piece. */
function KitPiece({
  element,
  top,
}: {
  element: KitElement
  /** The top piece, whose words sit above its ring rather than across its spoke. */
  top: boolean
}) {
  const { t } = useTranslation('creation')
  const text = useKitText()
  const id = useId()
  const stateId = `${id}-state`
  const factId = `${id}-fact`
  const open = t(`kit.open.${element.key}`)
  const ring = (
    <KitRing
      kitKey={element.key}
      state={element.state}
      fill={element.fill}
      // As a row the ring yields its room to the words, which wrap rather
      // than cut when the text is enlarged; on the orbit it is the picture.
      className="size-12 @xs:size-16 @md:size-20"
    />
  )
  const defaultKey = hasKitDefault(element.key) ? element.key : undefined
  const choosable = defaultKey !== undefined
  return (
    <div
      className={clsx(
        'group relative flex min-h-16 w-full items-center gap-3 rounded-2xl px-3 py-2 text-left @xs:gap-1.5 @xs:p-0',
        // The top piece's spoke runs straight down from its ring, so its words sit above the ring
        // rather than across the spoke.
        top ? '@xs:flex-col-reverse' : '@xs:flex-col',
        !choosable &&
          'hover:bg-row-bg-hover active:bg-row-bg-active @xs:hover:bg-transparent @xs:active:bg-transparent',
      )}
    >
      {choosable ? (
        <Link
          to={element.href}
          hash={KIT_HASH[element.key]}
          aria-label={open}
          aria-describedby={`${stateId} ${factId}`}
          className="duration-base ease-standard flex shrink-0 rounded-full transition-transform hover:scale-105 active:scale-100"
        >
          {ring}
        </Link>
      ) : (
        <span className="duration-base ease-standard shrink-0 rounded-full transition-transform @xs:group-hover:scale-105">
          {ring}
        </span>
      )}
      <span className="flex min-w-0 flex-1 flex-col items-start @xs:w-full @xs:items-center @xs:text-center">
        <span id={stateId} className="sr-only">
          {text.state(element)}
        </span>
        {defaultKey ? (
          <KitDefaultControl
            kit={defaultKey}
            name={text.name(element)}
            describedBy={`${stateId} ${factId}`}
            className="-ml-2 @xs:ml-0"
          />
        ) : (
          <Link
            to={element.href}
            hash={KIT_HASH[element.key]}
            aria-label={open}
            aria-describedby={`${stateId} ${factId}`}
            className={typographyStyles({
              variant: 'label',
              className:
                // The focus ring goes round the whole piece the link stretches over, not the
                // name: the app's own ring is unlayered, hence the `!` that turns it off there.
                'text-content-primary focus-visible:after:outline-focus-ring max-w-full min-w-0 break-keep after:absolute after:inset-0 after:rounded-2xl focus-visible:outline-none! focus-visible:after:outline-2 focus-visible:after:outline-offset-2 focus-visible:after:outline-solid @xs:truncate',
            })}
          >
            {text.name(element)}
          </Link>
        )}
        <span
          id={factId}
          className={typographyStyles({
            variant: 'meta',
            className: clsx(
              'break-words @xs:max-w-full @xs:truncate',
              element.state === 'ready' && 'text-content-secondary',
            ),
          })}
        >
          {text.fact(element)}
        </span>
      </span>
      {!choosable && (
        <ChevronRight
          aria-hidden="true"
          className="text-content-tertiary size-4 shrink-0 @xs:hidden"
        />
      )}
    </div>
  )
}
