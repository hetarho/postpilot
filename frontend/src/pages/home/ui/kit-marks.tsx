import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'
import { AlertTriangle, Plus } from 'lucide-react'
import type { KitReadiness } from '../model/readiness'
import { KIT_ICONS, KIT_TONES } from '../model/kit-tones'

/** The element's mark at its level: filled in its ink when ready, a ring of it while almost, a
 *  dashed hole while empty. The one to fill next carries a still accent ring and, behind it, the
 *  only motion on home — a breathing halo. */
export function KitMarker({
  item,
  next = false,
  className,
}: {
  item: KitReadiness
  next?: boolean
  className?: string
}) {
  const Icon = KIT_ICONS[item.element]
  const tone = KIT_TONES[item.element]
  return (
    <span
      aria-hidden="true"
      className={twMerge(
        'relative isolate flex size-7 shrink-0 items-center justify-center',
        className,
      )}
    >
      {next && (
        <span className="pointer-events-none absolute -inset-2.5 -z-10 opacity-70">
          <span className="bg-promo-aurora-1 animate-promo-glow absolute inset-0 rounded-full blur-md" />
        </span>
      )}
      <span
        className={clsx(
          'flex size-full items-center justify-center rounded-full',
          item.level === 'ready' && tone.fill,
          item.level === 'partial' && ['bg-surface-raised ring-2 ring-inset', tone.ring],
          item.level === 'empty' &&
            'border-content-tertiary/70 text-content-tertiary bg-surface-raised border-[1.5px] border-dashed',
          item.level === 'failed' && 'bg-notice-danger-bg text-notice-danger-fg',
          item.level === 'loading' && 'bg-badge-neutral-bg text-content-tertiary',
          next && 'outline-stroke-accent outline-2 outline-offset-2',
        )}
      >
        {item.level === 'empty' ? (
          <Plus className="size-3.5" />
        ) : item.level === 'failed' ? (
          <AlertTriangle className="size-3.5" />
        ) : (
          <Icon className="size-3.5" />
        )}
      </span>
    </span>
  )
}
