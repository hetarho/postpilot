import { useId, useState, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { ChevronDown } from 'lucide-react'
import { typographyStyles } from '../typography/typographyStyles'

/** A heading that opens and closes the region under it (WAI-APG disclosure): the heading's button
 *  carries `aria-expanded` and points at the region it controls, and the closed region is not
 *  rendered, so nothing inside it takes focus or is read.
 *
 *  Controlled with `open` and `onOpenChange`, or uncontrolled from `defaultOpen`. Opening or
 *  closing changes nothing but what is shown. */
export function Disclosure({
  title,
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  headingLevel = 2,
  aside,
  lead,
  children,
  className,
}: {
  title: ReactNode
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  headingLevel?: 2 | 3 | 4
  /** Beside the heading's button, outside it — a count, a status, the space's own actions. */
  aside?: ReactNode
  /** Under the heading row and always shown, open or closed — a field that acts on the region. */
  lead?: ReactNode
  children: ReactNode
  className?: string
}) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(defaultOpen)
  const open = controlledOpen ?? uncontrolledOpen
  const id = useId()
  const regionId = `${id}-region`
  const buttonId = `${id}-button`
  const Heading = `h${headingLevel}` as const
  const toggle = () => {
    const next = !open
    if (controlledOpen === undefined) setUncontrolledOpen(next)
    onOpenChange?.(next)
  }
  return (
    <section className={className}>
      <div className="flex items-center gap-2">
        <Heading className="min-w-0 flex-1">
          <button
            type="button"
            id={buttonId}
            aria-expanded={open}
            // Only while the region is there to point at: a closed one is not rendered.
            aria-controls={open ? regionId : undefined}
            onClick={toggle}
            className={typographyStyles({
              variant: 'title',
              className:
                'hover:text-content-primary flex min-h-11 w-full items-center gap-2 text-left',
            })}
          >
            <ChevronDown
              aria-hidden="true"
              className={clsx(
                'duration-fast ease-standard size-5 shrink-0 transition-transform',
                !open && '-rotate-90',
              )}
            />
            <span className="min-w-0 flex-1">{title}</span>
          </button>
        </Heading>
        {aside}
      </div>
      {lead}
      {open && (
        <div id={regionId} role="region" aria-labelledby={buttonId}>
          {children}
        </div>
      )}
    </section>
  )
}
