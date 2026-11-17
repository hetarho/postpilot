import type { ComponentType, ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import {
  AlertTriangle,
  Brain,
  Check,
  ChevronRight,
  LayoutList,
  Plus,
  Quote,
  ShieldCheck,
} from 'lucide-react'
import { TemplateOutline } from '@/entities/template'
import { Badge, Skeleton, Typography, typographyStyles } from '@/shared/ui'
import type { KitElement, KitLevel, KitReadiness } from '../model/readiness'
import type { WritingKit } from '../model/useWritingKit'

const ICONS: Record<KitElement, ComponentType<{ className?: string }>> = {
  template: LayoutList,
  voice: Quote,
  memory: Brain,
  guideline: ShieldCheck,
}

/** Where each layer leads: its own screen once it holds something, or straight into making the
 *  first one while it is empty. */
const DESTINATIONS: Record<
  KitElement,
  { ready: string; empty: { to: string; search?: Record<string, unknown> } }
> = {
  template: { ready: '/templates', empty: { to: '/templates/new' } },
  voice: { ready: '/voices', empty: { to: '/voices', search: { new: 1 } } },
  memory: { ready: '/memories', empty: { to: '/memories', search: { new: 1 } } },
  guideline: { ready: '/guidelines', empty: { to: '/guidelines', search: { new: 1 } } },
}

/** Guide: the readiness graphic — a sheet of paper that IS the post the kit makes. Each element
 *  of the kit is one layer of that post, drawn from the owner's own data: the template is the
 *  structure (its headings), the voice is the sentence (its own sample), the memories are the
 *  facts woven in, the guidelines are the rules in the margin. A layer that is not ready is drawn
 *  as its dashed outline, so the owner sees the hole in the page rather than a percentage. A spine
 *  down the left joins the layers, its node saying ready, almost or empty; the next layer to fill
 *  carries a still accent ring with a breathing halo behind it (motion only in the halo). The
 *  whole layer is the way to its screen. */
export function KitSheet({ data }: { data: WritingKit }) {
  const { t } = useTranslation('creation')
  const next = data.summary.settled ? data.summary.next : undefined
  return (
    <figure aria-labelledby="home-sheet-caption" className="relative isolate min-w-0">
      {/* The halo: the kit's warmth grows with what is ready — still when reduced motion asks. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -inset-4 -z-10 sm:-inset-8"
        style={{ opacity: 0.35 + 0.15 * data.summary.ready }}
      >
        <div className="bg-promo-aurora-1 animate-promo-glow absolute top-0 left-0 size-2/3 rounded-full blur-3xl" />
        <div className="bg-promo-aurora-2 animate-promo-glow absolute right-0 bottom-0 size-2/3 rounded-full blur-3xl [animation-delay:1.6s]" />
      </div>
      <div className="bg-surface-raised ring-divider rounded-3xl p-4 shadow-xl ring-1 sm:p-6">
        <figcaption
          id="home-sheet-caption"
          className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 px-1"
        >
          <Typography variant="eyebrow" as="span">
            {t('home.sheet')}
          </Typography>
          <span aria-hidden="true" className="flex gap-1.5">
            <span className="bg-divider size-2 rounded-full" />
            <span className="bg-divider size-2 rounded-full" />
            <span className="bg-divider size-2 rounded-full" />
          </span>
        </figcaption>
        <ol className="relative mt-3">
          {data.kit.map((item, index) => (
            <li
              key={item.element}
              className="animate-rise relative"
              style={{ animationDelay: `${index * 90}ms` }}
            >
              {/* The spine between this node and the next. */}
              {index < data.kit.length - 1 && (
                <span
                  aria-hidden="true"
                  className={clsx(
                    'absolute top-11 bottom-0 left-[1.6875rem] w-0.5 rounded-full sm:left-[1.9375rem]',
                    item.level === 'ready' ? 'bg-badge-done-bg/50' : 'bg-divider',
                  )}
                />
              )}
              <KitLayer item={item} data={data} next={next === item.element} />
            </li>
          ))}
        </ol>
      </div>
    </figure>
  )
}

function KitLayer({ item, data, next }: { item: KitReadiness; data: WritingKit; next: boolean }) {
  const { t } = useTranslation('creation')
  const Icon = ICONS[item.element]
  const filled = item.level === 'ready' || item.level === 'partial'
  const destination = filled
    ? { to: DESTINATIONS[item.element].ready }
    : DESTINATIONS[item.element].empty
  const name = t(`home.elements.${item.element}.name`)
  return (
    <Link
      to={destination.to}
      search={destination.search}
      aria-label={`${name} · ${levelText(t, item)}`}
      className={clsx(
        'group relative grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 rounded-2xl p-3 sm:gap-x-4 sm:p-4',
        'hover:bg-row-bg-hover active:bg-row-bg-active duration-fast ease-standard transition-colors',
        next && 'ring-stroke-accent bg-surface-raised ring-2',
      )}
    >
      {next && (
        <span
          aria-hidden="true"
          className="bg-promo-aurora-1 animate-promo-glow pointer-events-none absolute -inset-1 -z-10 rounded-3xl opacity-25 blur-xl"
        />
      )}
      <Node level={item.level} icon={Icon} />
      <span className="flex min-w-0 flex-col gap-2">
        <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <span
            className={typographyStyles({ variant: 'label', className: 'text-content-primary' })}
          >
            {name}
          </span>
          <span className={typographyStyles({ variant: 'meta', className: 'min-w-0' })}>
            {t(`home.elements.${item.element}.role`)}
          </span>
          <span className="ml-auto flex items-center gap-1">
            {next && <Badge tone="accent">{t('home.next')}</Badge>}
            <LevelBadge item={item} />
          </span>
        </span>
        <span className="block min-w-0">
          <LayerBody item={item} data={data} />
        </span>
        <span
          className={typographyStyles({
            variant: 'label',
            className:
              'text-link-fg group-hover:text-link-fg-hover inline-flex items-center gap-0.5 self-end',
          })}
        >
          {t(`home.open.${item.level === 'ready' ? 'ready' : filled ? 'partial' : 'empty'}`)}
          <ChevronRight aria-hidden="true" className="size-4" />
        </span>
      </span>
    </Link>
  )
}

function levelText(t: ReturnType<typeof useTranslation<'creation'>>['t'], item: KitReadiness) {
  if (item.level === 'partial' && item.percent !== undefined)
    return t('home.level.making', { percent: item.percent })
  if (item.level === 'loading') return '…'
  return t(`home.level.${item.level}`)
}

function LevelBadge({ item }: { item: KitReadiness }) {
  const { t } = useTranslation('creation')
  if (item.level === 'loading')
    return (
      <span aria-hidden="true" className="loading-reveal block">
        <Skeleton className="h-5 w-14" />
      </span>
    )
  const tone = ({ ready: 'done', partial: 'warning', empty: 'neutral', failed: 'danger' } as const)[
    item.level
  ]
  return <Badge tone={tone}>{levelText(t, item)}</Badge>
}

function Node({
  level,
  icon: Icon,
}: {
  level: KitLevel
  icon: ComponentType<{ className?: string }>
}) {
  return (
    <span
      aria-hidden="true"
      className={clsx(
        'relative z-10 flex size-8 shrink-0 items-center justify-center rounded-full',
        level === 'ready' && 'bg-badge-done-bg text-badge-done-fg',
        level === 'partial' && 'bg-notice-warning-bg text-notice-warning-fg',
        level === 'empty' && 'border-content-tertiary text-content-tertiary border-2 border-dashed',
        level === 'failed' && 'bg-notice-danger-bg text-notice-danger-fg',
        level === 'loading' && 'bg-badge-neutral-bg text-content-tertiary',
      )}
    >
      {level === 'ready' ? (
        <Check className="size-4" />
      ) : level === 'empty' ? (
        <Plus className="size-4" />
      ) : level === 'failed' ? (
        <AlertTriangle className="size-4" />
      ) : (
        <Icon className="size-4" />
      )}
    </span>
  )
}

/** The dashed outline of a layer that is missing: the shape it would take, and one line on what
 *  the page lacks without it. */
function Missing({ lines, children }: { lines: number[]; children: ReactNode }) {
  return (
    <span className="flex flex-col gap-2">
      {lines.map((width, index) => (
        <span
          key={index}
          aria-hidden="true"
          className="border-content-tertiary/50 block h-3 rounded-full border border-dashed"
          style={{ width: `${width}%` }}
        />
      ))}
      <span className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}>
        {children}
      </span>
    </span>
  )
}

function Lines({ widths }: { widths: number[] }) {
  return (
    <span aria-hidden="true" className="flex flex-col gap-1.5">
      {widths.map((width, index) => (
        <span
          key={index}
          className="bg-divider block h-2 rounded-full"
          style={{ width: `${width}%` }}
        />
      ))}
    </span>
  )
}

function LayerBody({ item, data }: { item: KitReadiness; data: WritingKit }) {
  const { t } = useTranslation('creation')
  if (item.level === 'loading')
    return (
      <span aria-hidden="true" className="loading-reveal flex flex-col gap-2">
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-4 w-1/2" />
      </span>
    )
  if (item.level === 'failed') return null
  switch (item.element) {
    case 'template':
      return data.template ? (
        <span className="flex flex-col gap-2">
          <span
            className={typographyStyles({
              variant: 'fieldTitle',
              className: 'text-content-primary break-words',
            })}
          >
            {data.template.first.name}
          </span>
          <TemplateOutline template={data.template.first} max={3} />
          {data.template.count > 1 && (
            <span className={typographyStyles({ variant: 'meta' })}>
              {t('home.elements.template.count', { count: data.template.count })}
            </span>
          )}
        </span>
      ) : (
        <Missing lines={[70, 45, 60]}>{t('home.elements.template.empty')}</Missing>
      )
    case 'voice':
      if (item.level === 'empty')
        return <Missing lines={[95, 85, 60]}>{t('home.elements.voice.empty')}</Missing>
      if (data.voice?.sample && item.level === 'ready')
        return (
          <span className="flex flex-col gap-2">
            <span
              className={typographyStyles({
                variant: 'fieldTitle',
                className: 'text-content-primary break-words',
              })}
            >
              “{data.voice.sample}”
            </span>
            {data.voice.phrases.length > 0 && (
              <span className="flex flex-wrap gap-1.5">
                {data.voice.phrases.map((phrase) => (
                  <span
                    key={phrase}
                    className={typographyStyles({
                      variant: 'meta',
                      className:
                        'bg-voice-habit-highlight text-voice-habit-foreground rounded-md px-2 py-0.5',
                    })}
                  >
                    {phrase}
                  </span>
                ))}
              </span>
            )}
            <span className={typographyStyles({ variant: 'meta' })}>
              {t('home.elements.voice.defaultName', { name: data.voice.voice.name })}
            </span>
          </span>
        )
      if (item.level === 'ready' && data.voice)
        return (
          <span className="flex flex-col gap-2">
            <Lines widths={[95, 88, 70]} />
            <span className={typographyStyles({ variant: 'body' })}>
              {t('home.elements.voice.noSample', { name: data.voice.voice.name })}
            </span>
          </span>
        )
      return (
        <span className="flex flex-col gap-2">
          <Lines widths={[95, 88, 70]} />
          {item.percent !== undefined && (
            <span
              aria-hidden="true"
              className="bg-meter-track block h-1.5 w-full overflow-hidden rounded-full"
            >
              <span
                className="bg-meter-fill block h-full rounded-full"
                style={{ width: `${item.percent}%` }}
              />
            </span>
          )}
          <span
            className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}
          >
            {t(
              item.percent === undefined
                ? 'home.elements.voice.noDefault'
                : item.percent >= 100
                  ? 'home.elements.voice.readyToMake'
                  : 'home.elements.voice.making',
            )}
          </span>
        </span>
      )
    case 'memory':
      return data.memories.count > 0 ? (
        <span className="flex flex-col gap-2">
          <span className="flex flex-wrap gap-1.5">
            {data.memories.shown.map((memory) => (
              <span
                key={memory.id}
                className={typographyStyles({
                  variant: 'body',
                  className:
                    'bg-origin-owner-highlight text-origin-owner-foreground line-clamp-1 max-w-full rounded-lg px-2.5 py-1 break-all',
                })}
              >
                {memory.text}
              </span>
            ))}
          </span>
          {data.memories.count > data.memories.shown.length && (
            <span className={typographyStyles({ variant: 'meta' })}>
              {t('home.elements.memory.more', {
                count: data.memories.count - data.memories.shown.length,
              })}
            </span>
          )}
        </span>
      ) : (
        <Missing lines={[40]}>{t('home.elements.memory.empty')}</Missing>
      )
    case 'guideline':
      return data.guidelines.shown.length > 0 ? (
        <span className="flex flex-col gap-2">
          <span className="flex flex-col gap-1">
            {data.guidelines.shown.map((rule) => (
              <span key={rule.key} className="flex min-w-0 items-start gap-2">
                <Check aria-hidden="true" className="text-badge-done-bg mt-1 size-4 shrink-0" />
                <span
                  className={typographyStyles({
                    variant: 'body',
                    className: 'text-content-primary line-clamp-1 min-w-0 flex-1',
                  })}
                >
                  {rule.text}
                </span>
                {rule.builtIn && (
                  <Badge tone="neutral" className="shrink-0">
                    {t('home.elements.guideline.defaultBadge')}
                  </Badge>
                )}
              </span>
            ))}
          </span>
          <span className={typographyStyles({ variant: 'meta' })}>
            {[
              t('home.elements.guideline.own', { count: data.guidelines.own }),
              t('home.elements.guideline.defaults', { count: data.guidelines.defaults }),
            ].join(' · ')}
          </span>
        </span>
      ) : (
        <Missing lines={[65, 50]}>{t('home.elements.guideline.empty')}</Missing>
      )
  }
}
