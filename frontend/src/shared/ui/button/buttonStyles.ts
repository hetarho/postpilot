import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export type ButtonVariant = 'cta' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'default' | 'icon'

const VARIANT_STYLES: Record<ButtonVariant, string> = {
  cta: 'bg-button-cta-bg text-button-cta-fg hover:bg-button-cta-bg-hover active:bg-button-cta-bg-active',
  secondary:
    'bg-button-secondary-bg text-button-secondary-fg hover:bg-button-secondary-bg-hover active:bg-button-secondary-bg-active',
  ghost:
    'bg-button-ghost-bg text-button-ghost-fg hover:bg-button-ghost-bg-hover active:bg-button-ghost-bg-active',
  danger:
    'bg-button-ghost-bg text-button-danger-quiet-fg hover:bg-button-danger-quiet-bg-hover active:scale-95',
}

const SIZE_STYLES: Record<ButtonSize, string> = {
  default: 'min-h-11 px-3',
  icon: 'size-11 shrink-0 p-0',
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
      'inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50',
      VARIANT_STYLES[variant],
      SIZE_STYLES[size],
      className,
    ),
  )
}
