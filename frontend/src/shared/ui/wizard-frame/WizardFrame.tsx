import { useEffect, useId, useRef, type ReactNode } from 'react'
import { GuideProgress, type GuideStep } from '../guide-progress/GuideProgress'
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
  /** Where the owner is, drawn the way every guided flow draws it (`GuideProgress`): one
   *  segment per screen the owner actually answers, and the position over the question. */
  progress: {
    steps: readonly GuideStep<string>[]
    current: string
    label: string
    position?: ReactNode
  }
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
      <GuideProgress
        steps={progress.steps}
        current={progress.current}
        ariaLabel={progress.label}
        className="mt-2"
      />
      {progress.position && (
        <p
          className={typographyStyles({
            variant: 'label',
            className: 'text-link-fg-current mt-6',
          })}
        >
          {progress.position}
        </p>
      )}
      {/* A stage heading takes focus for announcement on navigation; it is not a control. */}
      <h2
        ref={heading}
        id={titleId}
        tabIndex={-1}
        className={typographyStyles({
          variant: 'stepTitle',
          className: progress.position
            ? 'mt-1 text-balance focus:outline-none!'
            : 'mt-6 text-balance focus:outline-none!',
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
          {/* The reason names the held button for a screen reader only: a line that appeared
              above the row and vanished on the first tap jolted the whole bar (owner 2026-11-16). */}
          {next?.reason && (
            <div className="sr-only">
              <StepBarReason id={reasonId}>{next.reason}</StepBarReason>
            </div>
          )}
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
