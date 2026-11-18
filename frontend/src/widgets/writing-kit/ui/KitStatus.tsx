import { clsx } from 'clsx'
import { typographyStyles } from '@/shared/ui'
import type { KitKey } from '../model/kit'
import { useKitElement } from './useKitElement'
import { useKitText } from './useKitText'

/** The piece's state in words, and its one fact — under a kit screen's title and in the sheet
 *  홈 opens from the piece, so both say the same thing in the same shape. */
export function KitStatus({ kit, className }: { kit: KitKey; className?: string }) {
  const element = useKitElement(kit)
  const text = useKitText()
  return (
    <p className={clsx('flex min-w-0 flex-wrap items-baseline gap-x-2', className)} role="status">
      <span
        className={typographyStyles({
          variant: 'label',
          className: clsx(
            'shrink-0',
            element.state === 'ready' ? 'text-link-fg' : 'text-content-secondary',
          ),
        })}
      >
        {text.status(element)}
      </span>
      {element.state !== 'loading' &&
        element.state !== 'failed' &&
        element.fact.kind !== 'none' && (
          <span className={typographyStyles({ variant: 'meta', className: 'min-w-0 break-words' })}>
            {text.fact(element)}
          </span>
        )}
    </p>
  )
}
