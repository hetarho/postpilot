import type { CSSProperties } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Check, PenTool } from 'lucide-react'
import { Skeleton, typographyStyles } from '@/shared/ui'
import type { KitPart, KitPartKey, KitSummary } from '../model/readiness'
import { KIT_ICONS } from './kit-icons'
import { useKitStatusText } from './useKitStatusText'

/** Guide: the writing kit drawn as the post it makes — one sheet of paper, annotated. Each part
 *  of the kit draws its own piece of the page, so the owner sees what a part DOES before reading
 *  its name: 말투 inks the lines in the owner's own colour (as far as the voice is learned),
 *  템플릿 cuts the text into titled sections, 지침 rules the margin and ticks it, 기억 pins notes to
 *  the edge, and 쓰는 AI is the pen. A part not yet ready is drawn as a dashed outline of itself.
 *
 *  The annotations are the way in: each is a link to the screen where that part is made ready,
 *  with its name and its status in words — the drawing is decoration ('aria-hidden') and never the
 *  only signal. Wide enough, the annotations sit around the sheet with a leader line to the piece
 *  they name; narrower (a small phone, or text at 200%) they become the sheet's legend under it.
 *  Motion is the sheet's pieces arriving once, and a breathing halo behind the ONE part that is
 *  next — never a light travelling along a border. */
export function WritingKit({
  parts,
  summary,
  className,
}: {
  parts: Record<KitPartKey, KitPart>
  summary: KitSummary
  className?: string
}) {
  const { t } = useTranslation('creation')
  return (
    <figure aria-label={t('kit.figure')} className={clsx('@container w-full', className)}>
      <div
        className={clsx(
          'grid grid-cols-1 items-center gap-x-3 gap-y-5 @min-[16rem]:grid-cols-2',
          '@min-[21rem]:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_minmax(0,1fr)] @min-[21rem]:gap-x-0 @min-[21rem]:gap-y-4',
        )}
      >
        <div
          className={clsx(
            'mx-auto w-full max-w-56 @min-[16rem]:col-span-2',
            '@min-[21rem]:col-span-1 @min-[21rem]:col-start-2 @min-[21rem]:row-span-2 @min-[21rem]:row-start-1 @min-[21rem]:max-w-none',
          )}
        >
          <Manuscript parts={parts} complete={summary.phase === 'ready'} />
        </div>
        <Callout
          part={parts.template}
          next={summary.next}
          side="left"
          className="@min-[21rem]:col-start-1 @min-[21rem]:row-start-1"
        />
        <Callout
          part={parts.voice}
          next={summary.next}
          side="right"
          className="@min-[21rem]:col-start-3 @min-[21rem]:row-start-1"
        />
        <Callout
          part={parts.guideline}
          next={summary.next}
          side="left"
          className="@min-[21rem]:col-start-1 @min-[21rem]:row-start-2"
        />
        <Callout
          part={parts.memory}
          next={summary.next}
          side="right"
          className="@min-[21rem]:col-start-3 @min-[21rem]:row-start-2"
        />
        <Callout
          part={parts.model}
          next={summary.next}
          side="below"
          className="@min-[16rem]:col-span-2 @min-[16rem]:justify-self-center @min-[21rem]:col-span-1 @min-[21rem]:col-start-2 @min-[21rem]:row-start-3"
        />
      </div>
      <VideoNote part={parts.spoken} />
    </figure>
  )
}

/** One annotation: the part's ring, name and status, as one link to the part's own screen. */
function Callout({
  part,
  next,
  side,
  className,
}: {
  part: KitPart
  next: KitPartKey | undefined
  side: 'left' | 'right' | 'below'
  className?: string
}) {
  const { t } = useTranslation('creation')
  const statusText = useKitStatusText()
  const isNext = next === part.key
  // The leader line runs from the annotation to the sheet once the annotations sit beside it.
  const leader = (
    <span
      aria-hidden="true"
      className={clsx(
        'hidden shrink-0',
        side === 'below'
          ? '@min-[34rem]:block @min-[34rem]:h-3 @min-[34rem]:w-px'
          : '@min-[34rem]:block @min-[34rem]:h-px @min-[34rem]:w-4',
        part.state === 'ready' ? 'bg-stroke-accent' : 'bg-divider',
      )}
    />
  )
  return (
    <Link
      to={part.href}
      hash={part.hash}
      className={clsx(
        'group hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 min-w-0 items-center gap-2 rounded-xl py-1 pr-2 transition-colors',
        // Beside a narrow sheet the ring stands over its words; with room, it sits in the line.
        '@min-[21rem]:flex-col @min-[21rem]:gap-1 @min-[21rem]:px-1 @min-[21rem]:text-center',
        side === 'left' &&
          '@min-[34rem]:flex-row-reverse @min-[34rem]:gap-2 @min-[34rem]:pr-0 @min-[34rem]:text-right',
        side === 'right' &&
          '@min-[34rem]:flex-row @min-[34rem]:gap-2 @min-[34rem]:pl-0 @min-[34rem]:text-left',
        className,
      )}
    >
      {leader}
      <Ring part={part} next={isNext} />
      <span className="flex min-w-0 flex-col">
        <span
          className={typographyStyles({
            variant: 'label',
            className: 'text-content-primary break-keep',
          })}
        >
          {t(`kit.parts.${part.key}`)}
        </span>
        {part.state === 'loading' ? (
          <span aria-busy="true" className="loading-reveal mt-1 block">
            <Skeleton className="h-3 w-12" />
          </span>
        ) : (
          <span
            className={typographyStyles({
              variant: 'meta',
              className: clsx(
                // A long voice or model name stops at two lines rather than running into the sheet.
                'line-clamp-2 break-keep',
                part.state === 'ready' && 'text-content-secondary',
                isNext && 'text-link-fg',
              ),
            })}
          >
            {statusText(part)}
          </span>
        )}
        {part.attention ? (
          <span className={typographyStyles({ variant: 'meta', className: 'text-link-fg' })}>
            {t('kit.status.review', { count: part.attention })}
          </span>
        ) : null}
        <span className="sr-only">
          {', '}
          {t(`kit.state.${isNext ? 'next' : part.state}`)}
          {', '}
          {t(`kit.does.${part.key}`)}
        </span>
      </span>
    </Link>
  )
}

/** 목소리 reads videos, not posts: it is not part of the writing count, so it stands under the
 *  figure as a note — the same ring, smaller words — rather than as a sixth piece of the page. */
function VideoNote({ part }: { part: KitPart }) {
  const { t } = useTranslation('creation')
  const statusText = useKitStatusText()
  return (
    <Link
      to={part.href}
      className="group hover:bg-row-bg-hover active:bg-row-bg-active border-divider mx-auto mt-6 flex min-h-11 w-fit max-w-full items-center gap-2 rounded-full border py-1 pr-4 pl-1 transition-colors"
    >
      <Ring part={part} size="sm" />
      <span className={typographyStyles({ variant: 'meta', className: 'break-keep' })}>
        {t('kit.video')}
      </span>
      <span
        className={typographyStyles({
          variant: 'label',
          className: 'text-content-primary break-keep',
        })}
      >
        {t(`kit.parts.${part.key}`)}
      </span>
      {part.state === 'loading' ? (
        <Skeleton className="h-3 w-10" />
      ) : (
        <span className={typographyStyles({ variant: 'meta', className: 'break-keep' })}>
          {statusText(part)}
        </span>
      )}
      <span className="sr-only">
        {', '}
        {t(`kit.state.${part.state}`)}
        {', '}
        {t(`kit.does.${part.key}`)}
      </span>
    </Link>
  )
}

const RING_RADIUS = 16
const RING_LENGTH = 2 * Math.PI * RING_RADIUS

/** A part's state at a glance: a ring that fills with the part's real progress around its glyph —
 *  full and tinted when ready, a dashed outline when empty. The next part breathes a halo. */
export function Ring({
  part,
  next = false,
  size = 'md',
}: {
  part: Pick<KitPart, 'key' | 'state' | 'progress'>
  next?: boolean
  size?: 'sm' | 'md' | 'lg'
}) {
  const Icon = KIT_ICONS[part.key]
  const empty = part.state === 'empty' || part.state === 'failed'
  const ready = part.state === 'ready'
  return (
    <span
      aria-hidden="true"
      className={clsx(
        'relative inline-flex shrink-0 items-center justify-center',
        size === 'lg' ? 'size-14' : size === 'sm' ? 'size-8' : 'size-10',
      )}
    >
      {next && (
        <span className="bg-promo-stroke animate-promo-glow absolute -inset-1.5 rounded-full blur-md" />
      )}
      <span className="bg-surface-base absolute inset-0 rounded-full" />
      <svg viewBox="0 0 40 40" className="absolute inset-0 size-full -rotate-90">
        <circle
          cx="20"
          cy="20"
          r={RING_RADIUS}
          fill="none"
          strokeWidth="3"
          strokeDasharray={empty ? '2.5 3.5' : undefined}
          className={empty ? 'stroke-content-tertiary/50' : 'stroke-meter-track'}
        />
        {part.progress > 0 && (
          <circle
            cx="20"
            cy="20"
            r={RING_RADIUS}
            fill="none"
            strokeWidth="3"
            strokeLinecap="round"
            strokeDasharray={`${RING_LENGTH * Math.min(1, part.progress)} ${RING_LENGTH}`}
            className="stroke-meter-fill"
          />
        )}
      </svg>
      <span
        className={clsx(
          'relative flex items-center justify-center rounded-full',
          size === 'lg' ? 'size-10' : size === 'sm' ? 'size-6' : 'size-7',
          ready ? 'bg-badge-accent-bg text-badge-accent-fg' : 'text-content-tertiary',
        )}
      >
        <Icon
          aria-hidden="true"
          className={size === 'lg' ? 'size-5' : size === 'sm' ? 'size-3.5' : 'size-4'}
        />
      </span>
    </span>
  )
}

/** Line widths of the three sections' text, in % of the column. */
const SECTION_LINES = [
  [94, 100, 62],
  [100, 88, 70],
  [97, 100, 46],
] as const

/** The sheet. Sized in container units so it is the same drawing at every size. */
function Manuscript({
  parts,
  complete,
}: {
  parts: Record<KitPartKey, KitPart>
  complete: boolean
}) {
  const { voice, template, guideline, memory, model } = parts
  const totalLines = SECTION_LINES.flat().length
  const inked = Math.round(voice.progress * totalLines)
  const structured = template.state === 'ready'
  const drafting = template.state === 'partial'
  const ownRules = guideline.status.kind === 'ownAndDefaults' ? guideline.status.own : 0
  const builtInRules =
    guideline.status.kind === 'defaultsOnly'
      ? guideline.status.count
      : guideline.status.kind === 'ownAndDefaults'
        ? guideline.status.defaults
        : 0
  const ticks = Math.min(3, ownRules > 0 ? ownRules : builtInRules)
  const notes = memory.status.kind === 'count' ? Math.min(3, memory.status.count) : 0
  const pen = model.state === 'ready'
  let line = 0
  const arrive = (index: number): CSSProperties => ({ animationDelay: `${120 + index * 70}ms` })
  return (
    <div aria-hidden="true" className="@container relative aspect-[3/4] w-full">
      {/* The paper it sits on: a soft pool of the accent and the posts already written under it. */}
      <div className="bg-badge-accent-bg absolute -inset-[10cqw] -z-10 rounded-full opacity-70 blur-3xl" />
      <div className="bg-card-bg ring-divider absolute inset-0 translate-x-[3cqw] translate-y-[2cqw] rotate-[5deg] rounded-[6cqw] opacity-60 shadow-sm ring-1" />
      <div className="bg-card-bg ring-divider absolute inset-0 -translate-x-[2cqw] translate-y-[1cqw] -rotate-[3deg] rounded-[6cqw] opacity-80 shadow-sm ring-1" />
      <div className="bg-card-bg ring-divider absolute inset-0 rounded-[6cqw] shadow-lg ring-1" />
      {/* 지침: the margin rule, solid in the accent once the owner has a rule of their own. */}
      <div
        className={clsx(
          'absolute top-[9cqw] bottom-[22cqw] left-[9cqw] w-0 border-l-[0.8cqw]',
          ownRules > 0
            ? 'border-stroke-accent'
            : builtInRules > 0
              ? 'border-content-tertiary/40'
              : 'border-content-tertiary/40 border-dashed',
        )}
      />
      <div className="absolute top-[10cqw] right-[10cqw] left-[17cqw] flex flex-col">
        {/* 템플릿 owns the title area too. */}
        <span
          className={clsx(
            'animate-rise block h-[4.6cqw] w-[64%] rounded-full',
            structured ? 'bg-content-primary/80' : 'bg-content-tertiary/30',
          )}
          style={arrive(0)}
        />
        {SECTION_LINES.map((widths, section) => (
          <div
            key={section}
            className="animate-rise relative mt-[7cqw]"
            style={arrive(section + 1)}
          >
            {section < ticks && (
              <span
                className={clsx(
                  'absolute top-[-0.6cqw] -left-[10.4cqw] flex size-[5cqw] items-center justify-center rounded-full',
                  ownRules > 0
                    ? 'bg-badge-accent-bg text-badge-accent-fg'
                    : 'bg-badge-neutral-bg text-badge-neutral-fg',
                )}
              >
                <Check className="size-[3.6cqw]" strokeWidth={3} />
              </span>
            )}
            <span
              className={clsx(
                'block h-[3.8cqw] w-[44%] rounded-full',
                structured
                  ? 'bg-meter-fill'
                  : drafting
                    ? 'border-stroke-accent border-[0.5cqw] border-dashed'
                    : 'border-content-tertiary/40 border-[0.5cqw] border-dashed',
              )}
            />
            {widths.map((width) => {
              const own = line++ < inked
              return (
                <span
                  key={line}
                  className={clsx(
                    'mt-[3.2cqw] block h-[2.5cqw] rounded-full',
                    own ? 'bg-meter-fill/55' : 'bg-content-tertiary/25',
                  )}
                  style={{ width: `${width}%` }}
                />
              )
            })}
          </div>
        ))}
      </div>
      {/* 기억: notes pinned over the sheet's right edge. */}
      <div className="absolute top-[58cqw] -right-[6cqw] flex flex-col items-end">
        {notes > 0 ? (
          Array.from({ length: notes }, (_, index) => (
            <span
              key={index}
              className={clsx(
                'animate-rise bg-clip-amber/90 -mt-[5cqw] block size-[17cqw] rounded-[1.5cqw] p-[3cqw] shadow-md first:mt-0',
                index === 0 ? 'rotate-6' : index === 1 ? 'mr-[3cqw] -rotate-3' : 'rotate-2',
              )}
              style={arrive(5 + index)}
            >
              <span className="bg-clip-ink/30 block h-[1.6cqw] w-[80%] rounded-full" />
              <span className="bg-clip-ink/30 mt-[2cqw] block h-[1.6cqw] w-[55%] rounded-full" />
            </span>
          ))
        ) : (
          <span className="border-content-tertiary/40 bg-card-bg block size-[17cqw] rotate-6 rounded-[1.5cqw] border-[0.6cqw] border-dashed" />
        )}
      </div>
      {/* Every part ready: a seal pressed once onto the corner — an arrival, not a loop. */}
      {complete && (
        <span
          className="bg-button-cta-bg text-button-cta-fg animate-rise absolute -top-[5cqw] -right-[5cqw] flex size-[18cqw] -rotate-12 items-center justify-center rounded-full shadow-md"
          style={arrive(8)}
        >
          <Check className="size-[9cqw]" strokeWidth={3} />
        </span>
      )}
      {/* 쓰는 AI: the pen, resting where the writing goes on. */}
      <span
        className={clsx(
          'absolute right-[9cqw] bottom-[7cqw] flex size-[15cqw] items-center justify-center rounded-full',
          pen
            ? 'bg-badge-accent-bg text-badge-accent-fg shadow-md'
            : 'border-content-tertiary/40 text-content-tertiary border-[0.6cqw] border-dashed',
        )}
      >
        {pen && (
          <span className="bg-promo-stroke animate-promo-glow absolute -inset-[2cqw] -z-10 rounded-full opacity-60 blur-md" />
        )}
        <PenTool className="size-[7.5cqw]" />
      </span>
    </div>
  )
}
