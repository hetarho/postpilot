import { useId, type ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'
import { typographyStyles } from '../typography/typographyStyles'

/** Guide: one screen of a guided flow — ONE question, large, and the one task under it. It
 *  arrives once with a gentle slide (forward from the right, a step back from the left); key it by
 *  the step at the call site so each step gets its own entrance. Reduced motion holds it still.
 *
 *  The question is the section's heading (`h2` by default), so the outline reads the flow as a
 *  list of questions. */
export function StepScreen({
  eyebrow,
  title,
  direction = 'forward',
  headingLevel = 2,
  hidden = false,
  children,
  className,
}: {
  /** A screen the flow is not on. It stays MOUNTED — an upload or a half-typed answer on it keeps
   *  running — and is simply not rendered; showing it again replays the entrance. */
  hidden?: boolean
  /** A short position or context line over the question — "2 / 4". */
  eyebrow?: ReactNode
  title: ReactNode
  direction?: 'forward' | 'back'
  headingLevel?: 1 | 2
  children: ReactNode
  className?: string
}) {
  const id = useId()
  const Heading = headingLevel === 1 ? 'h1' : 'h2'
  return (
    <section
      aria-labelledby={id}
      hidden={hidden}
      className={twMerge(
        direction === 'back' ? 'animate-step-back' : 'animate-step-in',
        'flex min-w-0 flex-col',
        className,
      )}
    >
      {eyebrow && (
        <p className={typographyStyles({ variant: 'label', className: 'text-link-fg-current' })}>
          {eyebrow}
        </p>
      )}
      <Heading
        id={id}
        className={typographyStyles({ variant: 'stepTitle', className: 'mt-1 text-balance' })}
      >
        {title}
      </Heading>
      {children}
    </section>
  )
}
