import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { typographyStyles } from '@/shared/ui'
import { kitFact } from '../lib/kit-fact'
import type { KitElement } from '../model/readiness'
import { useWritingKit } from '../model/useWritingKit'
import { KitRing } from './KitRing'

/** Guide: the head of one kit element's own screen — its ring as the home graphic draws it, what
 *  it does to a post, and what it holds now, in the owner's words. The same three things the
 *  element's corner of the home says, so a screen opened from there reads as that corner, close
 *  up. Read from the lists the screen itself reads; it requests nothing of its own. */
export function KitSectionHead({
  element,
  ownerId,
  className,
}: {
  element: KitElement
  ownerId: string
  className?: string
}) {
  const { t } = useTranslation('creation')
  const kit = useWritingKit(ownerId)
  const entry = kit.entries[element]
  return (
    <div className={clsx('flex min-w-0 items-center gap-3 sm:gap-4', className)}>
      <KitRing reading={entry.reading} size="md" className="sm:size-14" />
      <div className="min-w-0">
        <p className={typographyStyles({ variant: 'title', className: 'text-balance' })}>
          {t(`kit.does.${element}`)}
        </p>
        <p
          className={typographyStyles({
            variant: 'meta',
            className: clsx(
              'mt-0.5 truncate',
              entry.reading.state === 'empty' ? 'text-content-tertiary' : 'text-content-secondary',
            ),
          })}
        >
          {kitFact(entry, t)}
        </p>
      </div>
    </div>
  )
}
