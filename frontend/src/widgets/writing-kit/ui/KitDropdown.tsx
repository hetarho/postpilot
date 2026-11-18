import { useRef, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { ChevronDown } from 'lucide-react'
import { Popover, Typography, type PopoverHandle } from '@/shared/ui'

/** Guide: a name on 홈's picture that is also the way to change it — the name and its chevron are
 *  ONE trigger, and what it opens is a panel anchored to it at every width, a phone's too, never a
 *  sheet. The chevron turns over while the panel is open, from the trigger's own `aria-expanded`.
 *  Only the icon beside the name navigates; the name never does.
 *
 *  The trigger keeps the name's own place in the line: on a coarse pointer it grows to the 44px
 *  target over the space around the name rather than pushing the words under it down.
 *
 *  `children` gets `chosen`, which closes the panel and puts focus back on the trigger — for a
 *  single pick once it is saved, so the line under the name shows the new answer. */
export function KitDropdown({
  name,
  label,
  triggerLabel,
  triggerClassName,
  wrap = false,
  describedBy,
  title,
  help,
  className,
  children,
}: {
  /** The visible words of the trigger. */
  name: ReactNode
  /** The panel's name, and the trigger's unless `triggerLabel` names it. */
  label: string
  /** The trigger's own name where it differs from the panel's; it carries the visible `name`. */
  triggerLabel?: string
  /** The trigger's own tone where it stands on another plane than the page. */
  triggerClassName?: string
  /** Lets a name too long for its place run onto a second line instead of being cut. */
  wrap?: boolean
  /** Ids of the lines that say the piece's state. */
  describedBy?: string
  title: string
  help: ReactNode
  /** On the dropdown's root, for its place in the caller's line. */
  className?: string
  children: (chosen: () => void) => ReactNode
}) {
  const popover = useRef<PopoverHandle>(null)
  return (
    <Popover
      ref={popover}
      label={label}
      triggerAttributes={{ 'aria-label': triggerLabel ?? label, 'aria-describedby': describedBy }}
      triggerLabel={
        <>
          <span className={wrap ? 'min-w-0 text-center break-keep' : 'min-w-0 truncate'}>
            {name}
          </span>
          <ChevronDown
            aria-hidden="true"
            className="text-content-tertiary duration-base ease-standard size-4 shrink-0 transition-transform group-aria-expanded:rotate-180 motion-reduce:transition-none"
          />
        </>
      }
      triggerVariant="ghost"
      triggerClassName={clsx(
        // The button's own label type (buttonStyles) is the name's; only its tone is the page's.
        'group text-content-primary min-h-8 max-w-full min-w-0 gap-0.5 px-2',
        '-my-1.5 pointer-coarse:-my-3 pointer-coarse:min-h-11',
        triggerClassName,
      )}
      className={clsx('max-w-full min-w-0', className)}
      placement="below"
      align="start"
    >
      {(close) => {
        const chosen = () => {
          close()
          popover.current?.focus()
        }
        return (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-0.5">
              <Typography variant="label" as="h2" className="text-content-primary">
                {title}
              </Typography>
              <Typography variant="meta" as="p" className="text-pretty">
                {help}
              </Typography>
            </div>
            {children(chosen)}
          </div>
        )
      }}
    </Popover>
  )
}
