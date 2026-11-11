import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export type ButtonVariant = 'cta' | 'secondary' | 'ghost' | 'danger' | 'scrim'
export type ButtonSize = 'default' | 'compact' | 'icon' | 'hero'

// Every variant carries an `active:` treatment. Tailwind compiles `hover:` to
// `@media (hover: hover)`, so a variant whose only fill lives behind `hover:` emits CSS a
// touchscreen never matches — it is invisible on the device this product is for
// (THEME-28). `danger` additionally takes a resting plane on a coarse pointer, because
// a destructive action that renders as bare red text does not read as a control at all.
const VARIANT_STYLES: Record<ButtonVariant, string> = {
  cta: 'bg-button-cta-bg text-button-cta-fg hover:bg-button-cta-bg-hover active:bg-button-cta-bg-active',
  secondary:
    'bg-button-secondary-bg text-button-secondary-fg hover:bg-button-secondary-bg-hover active:bg-button-secondary-bg-active',
  ghost:
    'bg-button-ghost-bg text-button-ghost-fg hover:bg-button-ghost-bg-hover active:bg-button-ghost-bg-active',
  danger:
    'bg-button-ghost-bg text-button-danger-quiet-fg pointer-coarse:bg-button-danger-quiet-bg-hover hover:bg-button-danger-quiet-bg-hover active:bg-button-danger-quiet-bg-hover',
  // A control standing ON footage — the preview's sound, refresh and info controls. It takes the
  // media scrim's plane at rest rather than the ghost's transparent one, because a glyph alone is
  // invisible over a bright frame and unreadable over a dark one (THEME-25).
  scrim:
    'bg-media-scrim-bg/60 text-media-scrim-fg hover:bg-media-scrim-bg/80 active:bg-media-scrim-bg/90',
}

/** The control's height follows the POINTER, not the device (THEME-23). Under a coarse pointer —
 *  a thumb — the 44px touch floor stays (`pointer-coarse:min-h-11`); under a fine pointer the
 *  control rests at 40px, the web's own norm for a button (WCAG 2.5.8 asks 24, Apple asks 44 for
 *  touch, Material draws 40 on a 48 target), because a 44px button under a mouse reads as a touch
 *  layout blown up. The primitive carries both values so no slice ever sizes a control.
 *
 *  Horizontal padding is a function of the height the control actually has, not of the padding
 *  that was written (THEME-23). The height floor overrides the computed height, so a
 *  `py-2` control ends up with ~10–12px of effective vertical padding. Pairing that with `px-3`
 *  gives a 1:1 box, and because text is far wider than it is tall a 1:1 control always reads
 *  squat. `px-4` restores the ~2:1 ratio; the committing action takes one step more so it is also
 *  the physically heavier target.
 *
 *  `compact` is the ONE documented step below the floor (32px, 36px on touch — still above the
 *  24px WCAG 2.5.8 minimum). It exists for a low-emphasis WAY OUT that shares a dock with the
 *  reading area it would otherwise cover — 둘 다 사용하지 않기 over an A/B draft. A committing action
 *  never takes it. Inside a `sm:` flex row it stretches back to its siblings' height, so the
 *  shorter box is a phone-only saving. */
function sizeStyles(size: ButtonSize, variant: ButtonVariant): string {
  if (size === 'icon') return 'size-10 pointer-coarse:size-11 shrink-0 p-0'
  // 32px against a 20px line box leaves 6px of effective vertical padding, so `px-4` keeps THEME-23's
  // 2 : 1 ratio rather than turning the shorter control into a square one.
  if (size === 'compact') return 'min-h-8 pointer-coarse:min-h-9 px-4'
  // Guide: the ONE big action a step screen ends on — 생성, 다음, 완성. 56px at every pointer, the
  // full width of the thumb zone it sits in, in the heaviest label the app has.
  if (size === 'hero') return 'min-h-14 px-7 text-[1.0625rem] font-bold'
  // Guide: every committing action is at least 48px, whatever the pointer (THEME-23 raised).
  return variant === 'cta'
    ? 'min-h-12 px-6 text-base font-bold'
    : 'min-h-10 pointer-coarse:min-h-11 px-5'
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
      // Guide: every button is a full pill — soft, unmistakably a button at any size — and a
      // press sinks it a touch (scale), the friendly feedback a thumb feels.
      'duration-fast ease-standard relative inline-flex items-center justify-center gap-2 rounded-full text-[0.9375rem] leading-snug font-semibold transition-[background-color,transform] active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50',
      // Stable CSS hooks give dock-contained buttons, links and file labels the same larger
      // targets without changing their pointer sizing anywhere else, including portalled panels.
      'ui-button',
      size === 'icon' && 'ui-button-icon',
      variant === 'cta' && 'ui-button-cta',
      variant === 'secondary' && 'ui-button-secondary',
      VARIANT_STYLES[variant],
      sizeStyles(size, variant),
      className,
    ),
  )
}
