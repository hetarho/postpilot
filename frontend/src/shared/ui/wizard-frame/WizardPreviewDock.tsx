import { useId, useState, type ReactNode } from 'react'
import { Sparkles } from 'lucide-react'
import { Button } from '../button/Button'
import { Sheet } from '../sheet/Sheet'
import { Typography } from '../typography/Typography'

/** A wizard's preview below `lg` (TMPL-75, CLIP-221): a button named `label` that floats at the
 *  viewport's foot while the step scrolls and settles above Back and next, opening the preview in
 *  a bottom sheet. It glows once each time an answer changes the preview — `changes` counts them —
 *  and the global reduced-motion rule ends that glow before it is seen. Pass it as the
 *  `WizardFrame`'s `beforeActions`. */
export function WizardPreviewDock({
  changes,
  label,
  closeLabel,
  children,
}: {
  changes: number
  label: string
  closeLabel: string
  children: ReactNode
}) {
  const id = useId()
  const [open, setOpen] = useState(false)
  const close = () => setOpen(false)
  return (
    <>
      <div className="pointer-events-none sticky bottom-4 z-10 mt-6 flex justify-end lg:hidden">
        <span className="pointer-events-auto relative inline-flex">
          {changes > 0 && (
            <span
              key={changes}
              aria-hidden
              data-testid="preview-glow"
              className="bg-promo-stroke animate-preview-glow absolute -inset-1.5 rounded-full opacity-0 blur-md"
            />
          )}
          <Button
            variant="secondary"
            onClick={() => setOpen(true)}
            className="relative rounded-full"
          >
            <Sparkles aria-hidden className="size-4" />
            {label}
          </Button>
        </span>
      </div>
      <Sheet
        open={open}
        labelledBy={`${id}-title`}
        size="wide"
        header={
          <Typography variant="title" id={`${id}-title`}>
            {label}
          </Typography>
        }
        footer={
          <Button variant="secondary" onClick={close}>
            {closeLabel}
          </Button>
        }
        onClose={close}
      >
        {children}
      </Sheet>
    </>
  )
}
