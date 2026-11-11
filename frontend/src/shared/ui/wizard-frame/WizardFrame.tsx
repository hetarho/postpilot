import { useEffect, useId, useRef, type ReactNode } from 'react'
import { ProgressBar } from '../progress-bar/ProgressBar'
import {
  StepBar,
  StepBarActions,
  StepBarBack,
  StepBarCta,
  StepBarReason,
} from '../step-bar/StepBar'
import { Typography } from '../typography/Typography'
import { typographyStyles } from '../typography/typographyStyles'

/** One screen of a guided flow (THEME-52, THEME-55, THEME-56): where the owner is, the one
 *  question the screen asks, its body, and the step bar every stepped screen shares (`StepBar`):
 *  ← with lower emphasis and the one emphasized next action. Focus moves to the heading only when
 *  the screen actually changes, never on the first paint.
 *
 *  Back and next used to sit at the foot of the page, under the body, so the wizard opened from
 *  the writing guide moved its owner from a docked bar to buttons that scrolled away (owner
 *  decision 2026-11-11). The frame is a flex column that takes the rest of its parent's height, so
 *  its parents down from the page's `main` must be flex columns too, ending on no bottom padding:
 *  then the bar meets the screen's bottom edge even under a short question. */
export function WizardFrame({
  screen,
  title,
  help,
  progress,
  back,
  next,
  barAction,
  barFooter,
  children,
}: {
  /** Identity of the screen on show; a change is a navigation and moves focus to the heading. */
  screen: string
  title: ReactNode
  help?: ReactNode
  /** Numbers only when the total is known; otherwise the words alone say what is under way. */
  progress: { text: string; label: string; current?: number; total?: number }
  back?: { label: string; onClick: () => void; disabled?: boolean }
  /** The screen's one big action, `forward` (→) when it only moves to the next screen. While it
   *  is held, `reason` says why in one line above it and names the button for a screen reader. */
  next?: {
    label: ReactNode
    forward?: boolean
    onClick: () => void
    disabled?: boolean
    pending?: boolean
    reason?: ReactNode
  }
  /** A secondary action in the bar's row, between ← and next — the phone's 미리보기. */
  barAction?: ReactNode
  /** What a flow the wizard sits in adds under the bar's row (setup's 지금은 건너뛰기). */
  barFooter?: ReactNode
  children?: ReactNode
}) {
  const titleId = useId()
  const reasonId = useId()
  const heading = useRef<HTMLHeadingElement>(null)
  const shown = useRef(screen)
  useEffect(() => {
    if (shown.current === screen) return
    shown.current = screen
    heading.current?.focus()
  }, [screen])
  return (
    <section aria-labelledby={titleId} className="flex min-w-0 flex-1 flex-col">
      <Typography variant="label" as="p" className="text-content-secondary">
        {progress.text}
      </Typography>
      {/* A count it does not know yet is said in words over a still, empty track. The job bar's
          indeterminate sweep means "something is running", and on a question that is only
          waiting for the owner's answer it read as a load that never ended. */}
      {progress.total ? (
        <ProgressBar
          label={progress.label}
          done={progress.current}
          total={progress.total}
          className="mt-2"
        />
      ) : (
        <div aria-hidden="true" className="relative mt-2 h-0">
          <div className="bg-meter-track h-progress-bar absolute inset-x-0 top-0" />
        </div>
      )}
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
      {/* `mb-8` is the gap a long screen ends on above the bar. */}
      <div className="mt-6 mb-8">{children}</div>
      {(back || next || barFooter) && (
        <StepBar>
          {next?.reason && <StepBarReason id={reasonId}>{next.reason}</StepBarReason>}
          {(back || next || barAction) && (
            <StepBarActions
              back={
                back && (
                  <StepBarBack label={back.label} onClick={back.onClick} disabled={back.disabled} />
                )
              }
            >
              {barAction}
              {next && (
                <StepBarCta
                  disabled={next.disabled}
                  pending={next.pending}
                  forward={next.forward}
                  aria-describedby={next.reason ? reasonId : undefined}
                  onClick={next.onClick}
                >
                  {next.label}
                </StepBarCta>
              )}
            </StepBarActions>
          )}
          {barFooter}
        </StepBar>
      )}
    </section>
  )
}
