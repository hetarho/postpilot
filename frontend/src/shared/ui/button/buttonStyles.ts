import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export type ButtonVariant = 'cta' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'default' | 'icon'

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
 *  so it is also the physically heavier target. */
function sizeStyles(size: ButtonSize, variant: ButtonVariant): string {
  if (size === 'icon') return 'size-11 shrink-0 p-0'
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
      'relative inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50',
      VARIANT_STYLES[variant],
      sizeStyles(size, variant),
      className,
    ),
  )
}
