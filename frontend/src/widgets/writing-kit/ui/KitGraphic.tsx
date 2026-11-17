import { useId } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Check, ChevronRight } from 'lucide-react'
import { typographyStyles } from '@/shared/ui'
import { kitPageIllustration as kitPage } from '@/shared/assets'
import { KIT_HREF, KIT_LOOK } from '../config/kit'
import { kitFact } from '../lib/kit-fact'
import { KIT_MAX_MARKS, type KitElement, type KitReading } from '../model/readiness'
import type { KitEntry, WritingKit } from '../model/useWritingKit'

type Corner = 'tl' | 'tr' | 'bl' | 'br'

/** The quarter of the ring each element owns (degrees clockwise from three o'clock) and the
 *  corner its name stands in: 말투 and 템플릿 above the page, 지침 and 기억 under it. */
const QUARTERS: readonly { key: KitElement; from: number; corner: Corner }[] = [
  { key: 'voices', from: 180, corner: 'tl' },
  { key: 'templates', from: 270, corner: 'tr' },
  { key: 'memories', from: 0, corner: 'br' },
  { key: 'guidelines', from: 90, corner: 'bl' },
]

const CENTER = 100
/** The ring the four quarters are drawn on, and the inner orbit their beads sit on. */
const ARC_RADIUS = 86
const ARC_WIDTH = 12
const BEAD_RADIUS = 70
/** Degrees between two quarters, wide enough that their round ends stay apart. */
const QUARTER_GAP = 16
const BEAD_STEP = 6.4

/** Guide: the home's graphic. The page the owner's posts are written on, held by a ring of four
 *  quarters — 말투, 템플릿, 지침, 기억. A quarter says how ready its element is (whole and in the
 *  element's hue once ready; a quieter hue while only stand-ins hold it, a voice still being made
 *  filled as far as its readiness share; a dotted outline while empty), and the beads inside it
 *  count what it holds, one per thing (hollow for a stand-in). The names stand in the four
 *  corners as the ways in, each with what the element holds in the owner's own words. The quarters
 *  draw themselves in once, on arrival; once all four are ready, a halo breathes behind the page.
 *  No stroke ever travels. */
export function KitGraphic({ kit, className }: { kit: WritingKit; className?: string }) {
  const { t } = useTranslation('creation')
  const total = QUARTERS.length
  const complete = kit.ready === total && !kit.pending && !kit.failed
  const corner = (position: Corner) => {
    const index = QUARTERS.findIndex((item) => item.corner === position)
    return (
      <KitLabel
        entry={kit.entries[QUARTERS[index].key]}
        align={position.endsWith('r') ? 'end' : 'start'}
        index={index}
      />
    )
  }
  return (
    <figure
      aria-label={t('kit.label')}
      className={clsx(
        'mx-auto grid w-full max-w-md grid-cols-2 gap-x-3 gap-y-1 lg:max-w-lg',
        className,
      )}
    >
      {corner('tl')}
      {corner('tr')}
      <div className="relative col-span-2 mx-auto aspect-square w-[min(100%,20rem)] sm:w-[min(100%,24rem)] lg:-my-4 lg:w-[min(100%,26rem)]">
        {/* The halo: only once the kit is complete, breathing on opacity and scale alone. */}
        <div
          aria-hidden="true"
          className={clsx(
            'absolute inset-[12%] rounded-full blur-2xl transition-opacity duration-700',
            'bg-[conic-gradient(from_315deg,var(--color-kit-voice),var(--color-kit-template),var(--color-kit-memory),var(--color-kit-guideline),var(--color-kit-voice))]',
            complete ? 'animate-kit-halo opacity-60' : 'opacity-0',
          )}
        />
        <svg viewBox="0 0 200 200" aria-hidden="true" className="absolute inset-0 size-full">
          {QUARTERS.map((quarter, index) => (
            <Quarter
              key={quarter.key}
              reading={kit.entries[quarter.key].reading}
              from={quarter.from}
              delay={index * 140}
            />
          ))}
        </svg>
        {/* The page itself, and under it, on the page, how much of the kit is ready. */}
        <div
          className={clsx(
            'bg-surface-raised absolute inset-[23%] flex flex-col items-center justify-center rounded-full shadow-md transition-[filter] duration-700',
            !kit.pending && kit.ready === 0 && 'grayscale-[55%]',
          )}
        >
          <img
            src={kitPage}
            alt=""
            width={440}
            height={440}
            className="-mt-[14%] size-[76%] object-contain"
            draggable={false}
          />
          <p
            role="status"
            className={typographyStyles({
              variant: 'label',
              className:
                'bg-surface-base text-content-primary absolute bottom-[9%] inline-flex min-h-8 items-center gap-1.5 rounded-full px-3.5 whitespace-nowrap shadow-sm',
            })}
          >
            {kit.pending ? (
              <span className="text-content-secondary">{t('kit.checking')}</span>
            ) : complete ? (
              <>
                <Check aria-hidden="true" className="text-link-fg-current size-4 stroke-[3]" />
                {t('kit.complete')}
              </>
            ) : (
              <>
                <span aria-hidden="true" className="flex items-baseline gap-1">
                  <span
                    className={typographyStyles({
                      variant: 'fieldTitle',
                      className: 'tabular-nums',
                    })}
                  >
                    {t('kit.readyShort', { count: kit.ready, total })}
                  </span>
                  <span className="text-content-secondary">{t('kit.readyWord')}</span>
                </span>
                <span className="sr-only">
                  {kit.ready === 0 ? t('kit.none') : t('kit.ready', { count: kit.ready, total })}
                </span>
              </>
            )}
          </p>
        </div>
      </div>
      {corner('bl')}
      {corner('br')}
    </figure>
  )
}

function Quarter({ reading, from, delay }: { reading: KitReading; from: number; delay: number }) {
  const gradient = useId()
  const look = KIT_LOOK[reading.key]
  const start = from + QUARTER_GAP / 2
  const end = from + 90 - QUARTER_GAP / 2
  const settled = !reading.pending && !reading.failed
  const empty = settled && reading.state === 'empty'
  const ready = settled && reading.state === 'ready'
  // A partial element without a share of its own (a draft, a 기본 지침) is the whole quarter, held
  // back; a voice being made fills as far as it has come.
  const reach =
    !settled || reading.state === 'empty'
      ? 0
      : ready || reading.share === 0
        ? 1
        : Math.max(0.06, reading.share / 100)
  const fillEnd = start + (end - start) * reach
  const beads = settled
    ? Math.min(KIT_MAX_MARKS, reading.own > 0 ? reading.own : reading.standIns)
    : 0
  const middle = from + 45
  const first = middle - ((beads - 1) * BEAD_STEP) / 2
  return (
    <g>
      <defs>
        <linearGradient
          id={gradient}
          gradientUnits="userSpaceOnUse"
          x1={polar(ARC_RADIUS, start).x}
          y1={polar(ARC_RADIUS, start).y}
          x2={polar(ARC_RADIUS, end).x}
          y2={polar(ARC_RADIUS, end).y}
        >
          <stop offset="0" style={{ stopColor: `var(${look.color})`, stopOpacity: 0.55 }} />
          <stop offset="1" style={{ stopColor: `var(${look.color})`, stopOpacity: 1 }} />
        </linearGradient>
      </defs>
      {/* An empty quarter is a dotted outline of the arc it will fill; any other has its track. */}
      <path
        d={arc(ARC_RADIUS, start, end)}
        fill="none"
        strokeWidth={empty ? 3 : ARC_WIDTH}
        strokeLinecap="round"
        className={empty ? 'stroke-content-tertiary' : 'stroke-meter-track'}
        strokeDasharray={empty ? '0.01 6.5' : undefined}
      />
      {reach > 0 && (
        <path
          d={arc(ARC_RADIUS, start, fillEnd)}
          fill="none"
          strokeWidth={ARC_WIDTH}
          strokeLinecap="round"
          pathLength={1}
          strokeDasharray="1 1"
          stroke={ready ? `url(#${gradient})` : undefined}
          className={clsx('animate-kit-draw', !ready && look.stroke)}
          strokeOpacity={ready ? 1 : 0.4}
          style={{ animationDelay: `${delay}ms` }}
        />
      )}
      {Array.from({ length: beads }, (_, index) => {
        const at = polar(BEAD_RADIUS, first + index * BEAD_STEP)
        return reading.own > 0 ? (
          <circle
            key={index}
            cx={at.x}
            cy={at.y}
            r={2.7}
            className={clsx('animate-kit-bead origin-center [transform-box:fill-box]', look.fill)}
            style={{ animationDelay: `${delay + 420 + index * 45}ms` }}
          />
        ) : (
          <circle
            key={index}
            cx={at.x}
            cy={at.y}
            r={2.2}
            fill="none"
            strokeWidth={1.3}
            className={clsx('animate-kit-bead origin-center [transform-box:fill-box]', look.stroke)}
            strokeOpacity={0.7}
            style={{ animationDelay: `${delay + 420 + index * 45}ms` }}
          />
        )
      })}
    </g>
  )
}

/** One element's corner: its state mark and name, then what it holds in the owner's words, then
 *  (where there is room) what it does to a post — the way in to the screen that makes it ready. */
function KitLabel({
  entry,
  align,
  index,
}: {
  entry: KitEntry
  align: 'start' | 'end'
  index: number
}) {
  const { t } = useTranslation('creation')
  const { key, state } = entry.reading
  const settled = !entry.reading.pending && !entry.reading.failed
  return (
    <Link
      to={KIT_HREF[key]}
      aria-describedby={`kit-${key}-does`}
      className={clsx(
        'group animate-rise hover:bg-row-bg-hover active:bg-row-bg-active -mx-2 flex min-h-11 min-w-0 flex-col justify-center gap-0.5 rounded-2xl px-2 py-1.5',
        align === 'end' ? 'items-end text-right' : 'items-start text-left',
      )}
      style={{ animationDelay: `${index * 80}ms` }}
    >
      <span className="flex min-w-0 items-center gap-1.5">
        <StateMark element={key} state={settled ? state : undefined} />
        <span className={typographyStyles({ variant: 'fieldTitle', className: 'truncate' })}>
          {t(`kit.name.${key}`)}
        </span>
        <span className="sr-only">{settled ? t(`kit.state.${state}`) : ''}</span>
        <ChevronRight
          aria-hidden="true"
          className="text-content-tertiary size-4 shrink-0 transition-transform group-hover:translate-x-0.5"
        />
      </span>
      <span
        className={typographyStyles({
          variant: 'meta',
          className: clsx(
            'line-clamp-1 max-w-full break-all',
            state === 'empty' ? 'text-content-tertiary' : 'text-content-secondary',
          ),
        })}
      >
        {kitFact(entry, t)}
      </span>
      <span id={`kit-${key}-does`} className="sr-only sm:not-sr-only">
        <span className={typographyStyles({ variant: 'meta', className: 'text-content-tertiary' })}>
          {t(`kit.does.${key}`)}
        </span>
      </span>
    </Link>
  )
}

/** The state in a glance, in the element's hue: a check once ready, a half-filled dot while
 *  stand-ins hold it, a dashed ring while empty. */
function StateMark({ element, state }: { element: KitElement; state?: KitReading['state'] }) {
  const look = KIT_LOOK[element]
  return (
    <span
      aria-hidden="true"
      className={clsx(
        'relative flex size-4 shrink-0 items-center justify-center overflow-hidden rounded-full',
        state === 'ready' && clsx(look.text, 'bg-current'),
        state === 'partial' && clsx(look.text, 'border-2 border-current'),
        (state === 'empty' || !state) && 'border-content-tertiary border-2 border-dashed',
      )}
    >
      {state === 'ready' && <Check className="text-button-cta-fg size-3 stroke-[3.5]" />}
      {state === 'partial' && <span className="absolute inset-x-0 bottom-0 h-1/2 bg-current" />}
    </span>
  )
}

function polar(radius: number, degrees: number) {
  const radians = (degrees * Math.PI) / 180
  return {
    x: Number((CENTER + radius * Math.cos(radians)).toFixed(3)),
    y: Number((CENTER + radius * Math.sin(radians)).toFixed(3)),
  }
}

function arc(radius: number, from: number, to: number): string {
  const a = polar(radius, from)
  const b = polar(radius, to)
  return `M ${a.x} ${a.y} A ${radius} ${radius} 0 ${to - from > 180 ? 1 : 0} 1 ${b.x} ${b.y}`
}
