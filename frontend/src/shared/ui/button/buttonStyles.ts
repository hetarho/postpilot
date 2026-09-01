import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export type ButtonVariant = 'cta' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'default' | 'compact' | 'icon'

// Every variant carries an `active:` treatment. Tailwind compiles `hover:` to
// `@media (hover: hover)`, so a variant whose only fill lives behind `hover:` emits CSS a
// touchscreen never matches — it is invisible on the device this product is for
// (design-language §6). `danger` additionally takes a resting plane on a coarse pointer, because
// a destructive action that renders as bare red text does not read as a control at all.
const VARIANT_STYLES: Record<ButtonVariant, string> = {
  cta: 'bg-button-cta-bg text-button-cta-fg hover:bg-button-cta-bg-hover active:bg-button-cta-bg-active',
  secondary:
    'bg-button-secondary-bg text-button-secondary-fg hover:bg-button-secondary-bg-hover active:bg-button-secondary-bg-active',
  ghost:
    'bg-button-ghost-bg text-button-ghost-fg hover:bg-button-ghost-bg-hover active:bg-button-ghost-bg-active',
  danger:
    'bg-button-ghost-bg text-button-danger-quiet-fg pointer-coarse:bg-button-danger-quiet-bg-hover hover:bg-button-danger-quiet-bg-hover active:bg-button-danger-quiet-bg-hover',
}

/** Horizontal padding is a function of the height the control actually has, not of the padding
 *  that was written (design-language §4.2). `min-h-11` is a 44px TOUCH FLOOR: it overrides the
 *  computed height, so a `py-2` control ends up with ~12px of effective vertical padding. Pairing
 *  that with `px-3` gives a 1:1 box, and because text is far wider than it is tall a 1:1 control
 *  always reads squat. `px-4` restores the ~2:1 ratio; the committing action takes one step more
 *  so it is also the physically heavier target.
 *
 *  `compact` is the ONE documented step below the 44px floor (36px, still far above the 24px WCAG
 *  2.5.8 minimum). It exists for a low-emphasis WAY OUT that shares a dock with the reading area
 *  it would otherwise cover — 둘 다 사용하지 않기 over an A/B draft. A committing action never takes
 *  it. Inside a `sm:` flex row it stretches back to its siblings' height, so the shorter box is a
 *  phone-only saving. */
function sizeStyles(size: ButtonSize, variant: ButtonVariant): string {
  if (size === 'icon') return 'size-11 shrink-0 p-0'
  // 36px against a 20px line box leaves 8px of effective vertical padding, so `px-4` keeps §4.2's
  // 2 : 1 ratio rather than turning the shorter control into a square one.
  if (size === 'compact') return 'min-h-9 px-4'
  return variant === 'cta' ? 'min-h-11 px-5' : 'min-h-11 px-4'
}

/** Shared visual contract for native buttons and elements that must retain their own
 * semantics, such as router links and the file picker's label. */
export function buttonStyles({
  variant = 'secondary',
  size = 'default',
  className,
}: {
  variant?: ButtonVariant
  size?: ButtonSize
  className?: string
} = {}) {
  return twMerge(
    clsx(
      // `leading-snug` rather than the 1.43 that `text-sm` carries: a button has NO vertical
      // padding, its height IS the `min-h` floor, so the one case that decides the box is a label
      // wrapping in a narrow column — and two 20px lines leave 2px of air inside 44px. On the
      // single line every other button has, `items-center` makes the tighter line box invisible.
      'relative inline-flex items-center justify-center gap-2 rounded-md text-sm leading-snug font-medium active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50',
      VARIANT_STYLES[variant],
      sizeStyles(size, variant),
      className,
    ),
  )
}
