import { useEffect, useId, useRef, type ReactNode } from 'react'
import { Button } from '../button/Button'
import { ProgressBar } from '../progress-bar/ProgressBar'
import { Typography } from '../typography/Typography'
import { typographyStyles } from '../typography/typographyStyles'

/** One screen of a guided flow (THEME-52, THEME-55, THEME-56): where the owner is, the one
 *  question the screen asks, its body, Back with lower emphasis and the one emphasized next
 *  action the caller supplies. Focus moves to the heading only when the screen actually changes,
 *  never on the first paint. */
export function WizardFrame({
  screen,
  title,
  help,
  progress,
  back,
  next,
  beforeActions,
  children,
}: {
  /** Identity of the screen on show; a change is a navigation and moves focus to the heading. */
  screen: string
  title: ReactNode
  help?: ReactNode
  /** Numbers only when the total is known; otherwise the words alone say what is under way. */
  progress: { text: string; label: string; current?: number; total?: number }
  back?: { label: string; onClick: () => void }
  next?: ReactNode
  /** Rendered between the body and the actions, so a `sticky bottom-*` child floats at the
   *  viewport's foot while the body scrolls and settles just above Back and next. */
  beforeActions?: ReactNode
  children?: ReactNode
}) {
  const titleId = useId()
  const heading = useRef<HTMLHeadingElement>(null)
  const shown = useRef(screen)
  useEffect(() => {
    if (shown.current === screen) return
    shown.current = screen
    heading.current?.focus()
  }, [screen])
  return (
    <section aria-labelledby={titleId} className="min-w-0">
      <Typography variant="label" as="p" className="text-content-secondary">
        {progress.text}
      </Typography>
      <ProgressBar
        label={progress.label}
        done={progress.current}
        total={progress.total}
        className="mt-2"
      />
      {/* A stage heading takes focus for announcement on navigation; it is not a control. */}
      <h2
        ref={heading}
        id={titleId}
        tabIndex={-1}
        className={typographyStyles({
          variant: 'stepTitle',
          className: 'mt-6 focus:outline-none!',
        })}
      >
        {title}
      </h2>
      {help && (
        <Typography variant="body" as="p" className="text-content-secondary mt-2">
          {help}
        </Typography>
      )}
      <div className="mt-6">{children}</div>
      {beforeActions}
      {(back || next) && (
        <div className="mt-8 flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          {back ? (
            <Button variant="ghost" onClick={back.onClick}>
              {back.label}
            </Button>
          ) : (
            <span />
          )}
          {next}
        </div>
      )}
    </section>
  )
}
