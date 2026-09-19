import type { ReactNode } from 'react'
import { Button, Typography } from '@/shared/ui'

/** A step the post has not reached yet. Never a disabled tab: the point of showing all three from
 *  the first screen is that the shape of the flow is visible, so an empty step says what it is
 *  waiting for and offers the way to the step that produces it. */
export function EmptyStep({
  children,
  goTo,
  goToLabel,
}: {
  children: ReactNode
  goTo: () => void
  goToLabel: string
}) {
  return (
    <div className="mt-10">
      <Typography variant="body" className="text-content-tertiary">
        {children}
      </Typography>
      <Button variant="secondary" className="mt-4" onClick={goTo}>
        {goToLabel}
      </Button>
    </div>
  )
}
