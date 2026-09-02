import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** The §3 type roles (design-language). `input` is deliberately absent: the 16px phone floor
 *  belongs to the field primitives, and exposing it here would invite callers to size fields. */
export type TypographyVariant =
  'display' | 'title' | 'fieldTitle' | 'body' | 'label' | 'meta' | 'eyebrow'

/** The one place the §3 recipes exist. Slices never compose raw size/weight/tracking utilities —
 *  `pnpm lint:style` rejects them outside shared/ui — so hierarchy cannot drift per call site. */
const VARIANT_STYLES: Record<TypographyVariant, string> = {
  display: 'text-2xl font-semibold tracking-tight',
  title: 'text-lg font-semibold tracking-tight',
  /* A field's own heading, where it stands beside the step title rather than under it: SMALLER
     than `title` so the step keeps the outline, HEAVIER so the field still reads as named and not
     as a caption. `label` cannot do this — it is `content-secondary` at a normal weight, which is
     a hint about a control, not the name of one. */
  fieldTitle: 'text-base font-bold tracking-tight',
  body: 'text-sm leading-relaxed',
  label: 'text-sm text-content-secondary',
  meta: 'text-xs text-content-tertiary',
  eyebrow: 'text-[10px] font-medium tracking-wide text-content-tertiary uppercase',
}

/** Shared type contract for elements that must keep their own component or semantics — a router
 *  link, a `dt`, a live region — mirroring how `buttonStyles` serves links. Colour conflicts
 *  resolve toward `className` (twMerge), so a nav link can keep its `link-*` tokens while taking
 *  the role's size. */
export function typographyStyles({
  variant,
  mono = false,
  className,
}: {
  variant: TypographyVariant
  /** Verbatim technical values — an account id, a model id — keep alignment-stable glyphs. */
  mono?: boolean
  className?: string
}) {
  return twMerge(clsx(VARIANT_STYLES[variant], mono && 'font-mono', className))
}
