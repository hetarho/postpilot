import type { CSSProperties } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ChevronDown, ChevronRight, PenLine } from 'lucide-react'
import { POST_LIST_PATH, rememberPostEntry } from '@/entities/post'
import { useSession } from '@/entities/session'
import { SurfaceButton, typographyStyles } from '@/shared/ui'
import { penIllustration } from '@/shared/assets'
import { KIT_HASH, type KitElement, type KitKey } from '../model/kit'
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
 *  The pen IS the page's 글쓰기. A piece with an account default (`onOpen`) opens the sheet that
 *  changes it, marked by a small ▾ beside its name; every other piece opens the screen that makes
 *  it ready.
 *
 *  One DOM in two layouts: a circle wherever the container has room for one (≥ 20rem, measured
 *  in rem so enlarged text falls back on its own), and otherwise a column — the pen as a wide
 *  button, then the pieces as rows — where nothing can overlap. */
export function WritingKit({
  elements,
  fill,
  complete,
  onOpen,
  className,
}: {
  elements: readonly KitElement[]
  /** The rings' fills, averaged: how strongly the halo glows. */
  fill: number
  complete: boolean
  /** Opens a piece's default in place instead of its screen; a piece absent here stays a link. */
  onOpen?: Partial<Record<KitKey, () => void>>
  className?: string
}) {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const remember = () =>
    rememberPostEntry(user?.id ?? '', {
      path: POST_LIST_PATH,
      section: 'post',
      filters: {},
      scrollY: 0,
    })
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

        {/* The pen: the screen's one 글쓰기, in a halo that grows with the kit. */}
        <div className="relative @xs:absolute @xs:top-1/2 @xs:left-1/2 @xs:-translate-1/2">
          <span
            aria-hidden="true"
            style={{ opacity: complete ? undefined : 0.12 + fill * 0.5 }}
            className={clsx(
              'bg-promo-stroke pointer-events-none absolute inset-0 hidden rounded-full blur-2xl @xs:block @xs:scale-150',
              complete && 'animate-promo-glow',
            )}
          />
          <Link
            to="/posts/new"
            search={{}}
            onClick={remember}
            className={clsx(
              typographyStyles({ variant: 'label', className: 'text-button-cta-fg' }),
              // The app's own focus outline (unlayered, hence the `!`), set clear of the orbit
              // pen's ring: the ring IS its border there, so a ring-coloured focus would not show.
              'bg-button-cta-bg hover:bg-button-cta-bg-hover active:bg-button-cta-bg-active duration-base ease-standard relative flex min-h-14 w-full items-center justify-center gap-2 rounded-full px-6 shadow-lg transition @xs:focus-visible:outline-offset-8!',
              '@xs:bg-surface-raised @xs:text-badge-accent-fg @xs:ring-button-cta-bg @xs:hover:bg-surface-raised @xs:active:bg-surface-raised @xs:size-24 @xs:flex-col @xs:gap-0 @xs:px-0 @xs:ring-4 @xs:hover:scale-105 @xs:active:scale-100 @md:size-32',
            )}
          >
            <img
              src={penIllustration}
              alt=""
              aria-hidden="true"
              className="hidden size-14 -translate-y-0.5 object-contain drop-shadow-md @xs:block @md:size-20"
            />
            <PenLine aria-hidden="true" className="size-5 @xs:hidden" />
            <span className="@xs:-mt-1">{t('kit.write')}</span>
          </Link>
        </div>

        <ol
          aria-label={t('kit.aria')}
          className="grid grid-cols-[minmax(0,1fr)] gap-1 @xs:pointer-events-none @xs:absolute @xs:inset-0 @xs:block"
        >
          {elements.map((element, index) => {
            const point = ORBIT[element.key]
            return (
              <li
                key={element.key}
                style={
                  {
                    '--kit-x': `${point.x}%`,
                    '--kit-y': `${point.y}%`,
                    animationDelay: `${index * 60}ms`,
                  } as CSSProperties
                }
                className={clsx(
                  'animate-rise @xs:pointer-events-auto @xs:absolute @xs:top-(--kit-y) @xs:left-(--kit-x) @xs:w-28 @xs:-translate-x-1/2 @md:w-36',
                  // Anchor the RING's centre on the orbit point: its words hang below it, or above
                  // it for the top piece. Half the ring: 2rem, and 2.5rem once it grows.
                  point.y < 30
                    ? '@xs:-translate-y-[calc(100%-2rem)] @md:-translate-y-[calc(100%-2.5rem)]'
                    : '@xs:-translate-y-8 @md:-translate-y-10',
                )}
              >
                <KitPiece element={element} top={point.y < 30} onOpen={onOpen?.[element.key]} />
              </li>
            )
          })}
        </ol>
      </div>
    </div>
  )
}

/** One piece on the orbit, or one row of the column: its ring, its name and its one fact. It
 *  opens the piece's default in a sheet when it has an `onOpen`, its screen otherwise. */
function KitPiece({
  element,
  top,
  onOpen,
}: {
  element: KitElement
  /** The top piece, whose words sit above its ring rather than across its spoke. */
  top: boolean
  onOpen?: () => void
}) {
  const text = useKitText()
  const className = clsx(
    'hover:bg-row-bg-hover active:bg-row-bg-active group flex min-h-16 w-full items-center gap-3 rounded-2xl px-3 py-2 text-left @xs:gap-1.5 @xs:p-0 @xs:hover:bg-transparent @xs:active:bg-transparent',
    // The top piece's spoke runs straight down from its ring, so its words sit above the ring
    // rather than across the spoke.
    top ? '@xs:flex-col-reverse' : '@xs:flex-col',
  )
  const body = (
    <>
      <span className="duration-base ease-standard rounded-full transition-transform @xs:group-hover:scale-105">
        <KitRing
          kitKey={element.key}
          state={element.state}
          fill={element.fill}
          // As a row the ring yields its room to the words, which wrap rather
          // than cut when the text is enlarged; on the orbit it is the picture.
          className="size-12 @xs:size-16 @md:size-20"
        />
      </span>
      <span className="flex min-w-0 flex-1 flex-col @xs:w-full @xs:items-center @xs:text-center">
        <span className="flex max-w-full min-w-0 items-center gap-0.5">
          <span
            className={typographyStyles({
              variant: 'label',
              className: 'text-content-primary min-w-0 break-keep @xs:truncate',
            })}
          >
            {text.name(element)}
          </span>
          {onOpen && (
            <ChevronDown aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
          )}
        </span>
        <span
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
      {!onOpen && (
        <ChevronRight
          aria-hidden="true"
          className="text-content-tertiary size-4 shrink-0 @xs:hidden"
        />
      )}
    </>
  )
  if (onOpen)
    return (
      <SurfaceButton
        presentation="content"
        aria-label={text.label(element)}
        aria-haspopup="dialog"
        onClick={onOpen}
        className={className}
      >
        {body}
      </SurfaceButton>
    )
  return (
    <Link
      to={element.href}
      hash={KIT_HASH[element.key]}
      aria-label={text.label(element)}
      className={className}
    >
      {body}
    </Link>
  )
}
