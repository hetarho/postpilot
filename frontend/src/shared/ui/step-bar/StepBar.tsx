import { forwardRef, type ReactNode } from 'react'
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import { ActionBar } from '../action-bar/ActionBar'
import { Button, type ButtonProps } from '../button/Button'
import { Typography } from '../typography/Typography'

/** Guide: the ONE bottom bar of every stepped screen — the writing guide, 글 다듬기, 글 완성's
 *  failure notice and the template wizards — so moving through any of them is the same gesture
 *  in the same place. Each screen used to draw its own: the guide's ← and big button, the
 *  revision dock as a floating composer with 확정하기 tucked into its heading, and the wizards'
 *  이전 / 다음 as plain buttons at the foot of the page (owner decision 2026-11-11).
 *
 *  It is the `step` dock (`ActionBar`): it meets the screen's bottom edge on a phone with rounded
 *  top corners and its padding adding the safe area, and floats as a rounded bar in the centred
 *  column from `sm:` up. Mount it as the LAST child of the screen's flex column, whose bottom
 *  padding is 0: `mt-auto` takes it to the bottom edge under a short screen.
 *
 *  Inside, from top to bottom: what the screen has to say about its actions (a failure, a
 *  refusal, the one-line reason the big action is held, a composer such as 글 다듬기's request
 *  field), then the `StepBarActions` row. Reasons stand ABOVE the row, so a software keyboard may
 *  hide a control but never why it is disabled (THEME-31). */
export function StepBar({
  ariaLabel,
  children,
  className,
}: {
  ariaLabel?: string
  children: ReactNode
  className?: string
}) {
  return (
    <ActionBar dock="step" ariaLabel={ariaLabel} className={className}>
      <div className="flex min-w-0 flex-col gap-3">{children}</div>
    </ActionBar>
  )
}

/** The bar's action row: the way back first, then at most one secondary action, then the screen's
 *  one big action (`StepBarCta`), which takes the rest of the row. */
export function StepBarActions({
  back,
  children,
  className,
}: {
  /** A `StepBarBack`, absent on a screen with nowhere to go back to. */
  back?: ReactNode
  children?: ReactNode
  className?: string
}) {
  return (
    <div className={twMerge('flex min-w-0 items-center gap-2', className)}>
      {back}
      {children}
    </div>
  )
}

/** The way back: a round ← whose name is the destination's word (이전). */
export function StepBarBack({
  label,
  onClick,
  disabled,
}: {
  label: string
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <Button
      variant="secondary"
      size="icon"
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
    >
      <ArrowLeft aria-hidden="true" className="size-5" />
    </Button>
  )
}

/** The screen's one big action — 다음, 생성, 확정하기, 저장 — filling what the row leaves. One that
 *  only moves to the next screen is `forward` and carries →. */
export const StepBarCta = forwardRef<
  HTMLButtonElement,
  Omit<ButtonProps, 'variant' | 'size'> & { forward?: boolean }
>(function StepBarCta({ className, forward = false, children, ...props }, ref) {
  return (
    <Button
      ref={ref}
      variant="cta"
      size="hero"
      className={twMerge('min-w-0 flex-1', className)}
      {...props}
    >
      {children}
      {forward && <ArrowRight aria-hidden="true" className="size-5" />}
    </Button>
  )
})

/** Why the big action is held, in one line above the row. Name it from the action with
 *  `aria-describedby={id}` so a screen reader hears the reason on the disabled button. */
export function StepBarReason({ id, children }: { id: string; children: ReactNode }) {
  return (
    <Typography variant="label" as="p" id={id} className="min-w-0">
      {children}
    </Typography>
  )
}
