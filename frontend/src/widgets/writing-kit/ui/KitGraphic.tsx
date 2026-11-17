import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ChevronRight } from 'lucide-react'
import { typographyStyles } from '@/shared/ui'
import { kitPageIllustration as kitPage } from '@/shared/assets'
import { KIT_HREF, KIT_LOOK } from '../config/kit'
import { kitFact } from '../lib/kit-fact'
import { KIT_MAX_MARKS, type KitElement } from '../model/readiness'
import type { KitEntry, WritingKit } from '../model/useWritingKit'

/** The quarter of the ring each element owns, clockwise from the top-left, and which corner its
 *  name stands in. 말투 and 템플릿 above the page, 지침 and 기억 under it. */
const QUARTERS: readonly { key: KitElement; from: number; corner: 'tl' | 'tr' | 'bl' | 'br' }[] = [
  { key: 'voices', from: 180, corner: 'tl' },
  { key: 'templates', from: 270, corner: 'tr' },
  { key: 'memories', from: 0, corner: 'br' },
  { key: 'guidelines', from: 90, corner: 'bl' },
]

const CENTER = 100
const RADIUS = 84
const QUARTER_GAP = 7
const MARK_GAP = 2.4

/** Guide: the home's graphic. The page the owner's posts are written on, held by one ring of four
 *  quarters — 말투, 템플릿, 지침, 기억 — each quarter drawing one mark per thing the owner made in
 *  that element's hue (a voice still being made fills by its share; what only stands in, a draft
 *  or a 기본 지침, is a quieter mark; an empty quarter is dashed). The names stand in the four
 *  corners as the ways in, each with what the element holds in the owner's own words. Once all
 *  four are ready, a halo breathes behind the page; the marks draw themselves in once, on arrival.
 *  No stroke ever travels. */
export function KitGraphic({ kit, className }: { kit: WritingKit; className?: string }) {
  const { t } = useTranslation('creation')
  const complete = kit.ready === QUARTERS.length && !kit.pending
  const corner = (position: 'tl' | 'tr' | 'bl' | 'br') => {
    const quarter = QUARTERS.find((item) => item.corner === position)!
    return (
      <KitLabel
        entry={kit.entries[quarter.key]}
        align={position.endsWith('r') ? 'end' : 'start'}
        index={QUARTERS.indexOf(quarter)}
      />
    )
  }
  return (
    <figure
      aria-label={t('kit.label')}
      className={clsx('mx-auto grid w-full max-w-md grid-cols-2 gap-x-4 lg:max-w-lg', className)}
    >
      {corner('tl')}
      {corner('tr')}
      <div className="relative col-span-2 mx-auto my-2 aspect-square w-[min(100%,22rem)] sm:w-[min(100%,26rem)]">
        {/* The halo: only once the kit is complete, breathing on opacity and scale alone. */}
        <div
          aria-hidden="true"
          className={clsx(
            'absolute inset-[14%] rounded-full blur-2xl transition-opacity duration-700',
            'bg-[radial-gradient(circle,var(--color-kit-voice),var(--color-kit-template)_45%,transparent_72%)]',
            complete ? 'animate-kit-halo opacity-70' : 'opacity-0',
          )}
        />
        <svg viewBox="0 0 200 200" aria-hidden="true" className="absolute inset-0 size-full">
          {QUARTERS.map((quarter) => (
            <Quarter
              key={quarter.key}
              entry={kit.entries[quarter.key]}
              from={quarter.from}
              delay={QUARTERS.indexOf(quarter) * 120}
            />
          ))}
        </svg>
        <div className="absolute inset-[19%] flex flex-col items-center justify-center">
          <div
            className={clsx(
              'bg-surface-raised flex aspect-square w-full items-center justify-center overflow-hidden rounded-full shadow-md',
              !kit.pending && kit.ready === 0 && 'grayscale-[60%]',
            )}
          >
            <img
              src={kitPage}
              alt=""
              width={512}
              height={512}
              className="size-[86%] object-contain"
              draggable={false}
            />
          </div>
          <p
            className={typographyStyles({
              variant: 'label',
              className:
                'bg-surface-raised text-content-primary -mt-4 inline-flex items-baseline gap-1 rounded-full px-3 py-1 shadow-sm',
            })}
          >
            <span
              className={typographyStyles({ variant: 'fieldTitle', className: 'tabular-nums' })}
            >
              {kit.pending
                ? '–'
                : t('kit.readyShort', { count: kit.ready, total: QUARTERS.length })}
            </span>
            <span className="text-content-secondary">{t('kit.readyWord')}</span>
          </p>
        </div>
      </div>
      {corner('bl')}
      {corner('br')}
    </figure>
  )
}

function Quarter({ entry, from, delay }: { entry: KitEntry; from: number; delay: number }) {
  const look = KIT_LOOK[entry.reading.key]
  const start = from + QUARTER_GAP / 2
  const end = from + 90 - QUARTER_GAP / 2
  const { marks, pending } = entry.reading
  const empty =
    !pending && (marks.kind === 'share' ? marks.percent === 0 : marks.own + marks.assumed === 0)
  const drawn: { from: number; to: number; own: boolean }[] = []
  // A share is drawn in the element's hue at full strength only once the element is ready: a
  // voice with all its material but no analysis yet is a full ring, held back.
  if (!pending && marks.kind === 'share' && marks.percent > 0)
    drawn.push({
      from: start,
      to: start + ((end - start) * marks.percent) / 100,
      own: entry.reading.state === 'ready',
    })
  if (!pending && marks.kind === 'count') {
    const total = Math.min(KIT_MAX_MARKS, marks.own + marks.assumed)
    const span = (end - start - MARK_GAP * (total - 1)) / Math.max(total, 1)
    for (let index = 0; index < total; index++) {
      const at = start + index * (span + MARK_GAP)
      drawn.push({ from: at, to: at + span, own: index < Math.min(marks.own, total) })
    }
  }
  return (
    <g>
      <path
        d={arc(start, end)}
        fill="none"
        strokeWidth={14}
        strokeLinecap="round"
        className={empty ? 'stroke-content-tertiary' : 'stroke-meter-track'}
        strokeDasharray={empty ? '2 7' : undefined}
        strokeOpacity={empty ? 0.6 : 1}
      />
      {drawn.map((mark, index) => (
        <path
          key={index}
          d={arc(mark.from, mark.to)}
          fill="none"
          strokeWidth={14}
          // Several marks keep square ends, so each stays one countable mark.
          strokeLinecap={drawn.length > 1 ? 'butt' : 'round'}
          pathLength={1}
          strokeDasharray="1 1"
          className={clsx(
            'animate-kit-draw',
            mark.own || marks.kind === 'share' ? look.stroke : 'stroke-content-tertiary',
          )}
          strokeOpacity={mark.own ? 1 : marks.kind === 'share' ? 0.4 : 0.5}
          style={{ animationDelay: `${delay + index * 40}ms` }}
        />
      ))}
    </g>
  )
}

/** One element's corner: its name in its hue, then what it holds — the way in to the screen that
 *  makes it ready. */
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
  const look = KIT_LOOK[key]
  return (
    <Link
      to={KIT_HREF[key]}
      aria-describedby={`kit-${key}-does`}
      className={clsx(
        'group animate-rise hover:bg-row-bg-hover active:bg-row-bg-active -mx-2 flex min-h-11 min-w-0 flex-col justify-center gap-0.5 rounded-xl px-2 py-2',
        align === 'end' ? 'items-end text-right' : 'items-start text-left',
      )}
      style={{ animationDelay: `${index * 80}ms` }}
    >
      <span className="flex min-w-0 items-center gap-1.5">
        <span
          aria-hidden="true"
          className={clsx(
            'size-2.5 shrink-0 rounded-full',
            state === 'empty'
              ? 'border-content-tertiary border-2 border-dashed'
              : clsx(look.text, 'bg-current'),
          )}
        />
        <span className={typographyStyles({ variant: 'fieldTitle', className: 'truncate' })}>
          {t(`kit.name.${key}`)}
        </span>
        <ChevronRight
          aria-hidden="true"
          className="text-content-tertiary size-4 shrink-0 transition-transform group-hover:translate-x-0.5"
        />
      </span>
      <span
        className={typographyStyles({
          variant: 'meta',
          className: clsx(
            'line-clamp-2 break-words',
            state === 'empty' ? 'text-content-tertiary' : 'text-content-secondary',
          ),
        })}
      >
        {kitFact(entry, t)}
      </span>
      <span id={`kit-${key}-does`} className="sr-only lg:not-sr-only">
        <span className={typographyStyles({ variant: 'meta', className: 'text-content-tertiary' })}>
          {t(`kit.does.${key}`)}
        </span>
      </span>
    </Link>
  )
}

function arc(from: number, to: number): string {
  const point = (degrees: number) => {
    const radians = (degrees * Math.PI) / 180
    return `${(CENTER + RADIUS * Math.cos(radians)).toFixed(3)} ${(CENTER + RADIUS * Math.sin(radians)).toFixed(3)}`
  }
  return `M ${point(from)} A ${RADIUS} ${RADIUS} 0 ${to - from > 180 ? 1 : 0} 1 ${point(to)}`
}
