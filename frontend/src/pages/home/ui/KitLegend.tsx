import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ChevronRight } from 'lucide-react'
import { KIT_ILLUSTRATIONS } from '@/shared/assets'
import { Skeleton, Typography, typographyStyles } from '@/shared/ui'
import type { KitElement, KitReadiness } from '../model/readiness'
import type { WritingKit } from '../model/useWritingKit'
import { KitMarker } from './kit-marks'

/** Where each element leads: its own screen once it holds something, or straight into making the
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

/** Guide: the sheet's key — the four kit elements as the objects they are (a laid-out sheet, a
 *  pen, a pinned photo, a ruler and stamp), each with what it does to a post and where it stands
 *  in the owner's words. An object not ready yet is drawn faded; the one to fill next carries the
 *  sheet's own marker and a filled action. Every row opens the element's screen. */
export function KitLegend({ data, className }: { data: WritingKit; className?: string }) {
  const { t } = useTranslation('creation')
  const next = data.summary.settled ? data.summary.next : undefined
  return (
    <nav aria-labelledby="home-kit-legend" className={clsx('min-w-0', className)}>
      <Typography variant="groupLabel" as="h2" id="home-kit-legend">
        {t('home.legend')}
      </Typography>
      <ul className="divide-divider mt-1 divide-y">
        {data.kit.map((item) => (
          <li key={item.element}>
            <LegendRow item={item} data={data} next={next === item.element} />
          </li>
        ))}
      </ul>
    </nav>
  )
}

function LegendRow({ item, data, next }: { item: KitReadiness; data: WritingKit; next: boolean }) {
  const { t } = useTranslation('creation')
  const filled = item.level === 'ready' || item.level === 'partial'
  const destination = filled
    ? { to: DESTINATIONS[item.element].ready }
    : DESTINATIONS[item.element].empty
  const name = t(`home.elements.${item.element}.name`)
  const role = t(`home.elements.${item.element}.role`)
  const state = stateText(t, item, data)
  const action =
    item.level === 'empty' || item.level === 'partial' ? t(`home.action.${item.level}`) : undefined
  return (
    <Link
      to={destination.to}
      search={destination.search}
      aria-label={[name, role, state, action].filter(Boolean).join(' · ')}
      className="group hover:bg-row-bg-hover active:bg-row-bg-active duration-fast ease-standard -mx-2 flex min-h-18 items-center gap-3 rounded-2xl px-2 py-2.5 transition-colors"
    >
      <span className="relative shrink-0">
        <img
          src={KIT_ILLUSTRATIONS[item.element]}
          alt=""
          width={56}
          height={56}
          className={clsx(
            'size-14 object-contain transition-[opacity,filter]',
            item.level !== 'ready' && 'opacity-55 saturate-50',
          )}
        />
        <KitMarker
          item={item}
          next={next}
          className="ring-surface-base absolute -right-1 -bottom-1 size-6 rounded-full ring-2"
        />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="flex min-w-0 flex-wrap items-baseline gap-x-2">
          <span
            className={typographyStyles({ variant: 'label', className: 'text-content-primary' })}
          >
            {name}
          </span>
          <span className={typographyStyles({ variant: 'meta' })}>{role}</span>
        </span>
        {item.level === 'loading' ? (
          <span className="loading-reveal mt-1 block">
            <Skeleton className="h-4 w-2/3" />
          </span>
        ) : (
          <span
            className={typographyStyles({
              variant: 'body',
              className: clsx(
                'line-clamp-2',
                item.level === 'failed' ? 'text-notice-danger-fg' : 'text-content-secondary',
              ),
            })}
          >
            {state}
          </span>
        )}
      </span>
      {action ? (
        <span
          className={typographyStyles({
            variant: 'label',
            className: clsx(
              'inline-flex shrink-0 items-center gap-0.5 rounded-full py-1 pr-2 pl-3',
              next
                ? 'bg-badge-accent-bg text-badge-accent-fg'
                : 'text-link-fg group-hover:text-link-fg-hover',
            ),
          })}
        >
          {action}
          <ChevronRight aria-hidden="true" className="size-4" />
        </span>
      ) : (
        <ChevronRight aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
      )}
    </Link>
  )
}

function stateText(
  t: ReturnType<typeof useTranslation<'creation'>>['t'],
  item: KitReadiness,
  data: WritingKit,
): string {
  if (item.level === 'loading') return t('home.level.loading')
  if (item.level === 'failed') return t('home.level.failed')
  switch (item.element) {
    case 'template':
      if (item.level !== 'ready' || !data.template) return t('home.elements.template.empty')
      return data.template.count > 1
        ? t('home.elements.template.readyMore', {
            name: data.template.first.name,
            count: data.template.count - 1,
          })
        : t('home.elements.template.ready', { name: data.template.first.name })
    case 'voice':
      if (item.level === 'ready' && data.voice)
        return t('home.elements.voice.ready', { name: data.voice.voice.name })
      if (item.level === 'partial')
        return item.percent === undefined
          ? t('home.elements.voice.noDefault')
          : item.percent >= 100
            ? t('home.elements.voice.readyToMake')
            : t('home.elements.voice.making', { percent: item.percent })
      return t('home.elements.voice.empty')
    case 'memory':
      return item.level === 'ready'
        ? t('home.elements.memory.ready', { count: data.memories.count })
        : t('home.elements.memory.empty')
    case 'guideline':
      if (item.level === 'ready')
        return t('home.elements.guideline.ready', {
          own: data.guidelines.own,
          defaults: data.guidelines.defaults,
        })
      if (item.level === 'partial')
        return t('home.elements.guideline.partial', { count: data.guidelines.defaults })
      return t('home.elements.guideline.empty')
  }
}
