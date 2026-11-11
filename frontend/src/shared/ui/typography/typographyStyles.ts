import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'

/** The THEME-19 type roles. `input` is deliberately absent: the 16px phone floor
 *  belongs to the field primitives, and exposing it here would invite callers to size fields. */
export type TypographyVariant =
  | 'launch'
  | 'promoDisplay'
  | 'hero'
  | 'largeTitle'
  | 'display'
  | 'stepTitle'
  | 'title'
  | 'fieldTitle'
  | 'body'
  | 'label'
  | 'meta'
  | 'eyebrow'
  | 'figure'
  | 'groupLabel'

/** The one place the THEME-19 recipes exist. Slices never compose raw size/weight/tracking utilities —
 *  `pnpm lint:style` rejects them outside shared/ui — so hierarchy cannot drift per call site. */
const VARIANT_STYLES: Record<TypographyVariant, string> = {
  /* The one figure a PROMOTIONAL surface leads with — a plan's price (THEME-37). Larger than the
     screen's `display` on purpose: on a surface whose job is to be chosen from, the thing being
     compared outranks the page's own name. It is not a heading and never carries prose; a slice
     outside the plan ladder reaching for it is the promotional exception spreading. */
  // The named promotional exception: a campaign headline, never an ordinary page title.
  launch: 'text-4xl font-semibold tracking-tight leading-tight sm:text-6xl',
  promoDisplay: 'text-4xl font-bold tracking-tight leading-tight sm:text-6xl',
  // Five-digit KRW prices still fit a narrow card with text enlarged to 200%.
  hero: 'text-xl font-bold tracking-tight sm:text-4xl',
  /* Guide: the large page title a channel or library opens on (the iOS large title), and the one
     question a guided step asks. The heaviest weight in the app, so the screen's one question is
     the first thing read. */
  largeTitle: 'text-large-title font-extrabold tracking-tight sm:text-large-title-wide',
  display: 'text-3xl font-bold tracking-tight leading-tight sm:text-4xl',
  stepTitle: 'text-step-title font-bold tracking-tight leading-tight sm:text-step-title-wide',
  title: 'text-title font-bold tracking-tight leading-snug sm:text-2xl',
  /* A field's own heading, where it stands beside the step title rather than under it: SMALLER
     than `title` so the step keeps the outline, HEAVIER so the field still reads as named and not
     as a caption. `label` cannot do this — it is `content-secondary` at a normal weight, which is
     a hint about a control, not the name of one. */
  fieldTitle: 'text-lg font-bold tracking-tight leading-snug',
  /* Guide: 17px reading text on every width — a busy owner reads this at arm's length. */
  body: 'text-body leading-relaxed',
  label: 'text-label font-semibold text-content-secondary',
  meta: 'text-meta text-content-tertiary',
  eyebrow: 'text-xs font-semibold tracking-wide text-content-tertiary uppercase',
  /* Guide: the one big number a hero card is about — the credits left. Not a heading: the card's
     own label names it. */
  figure: 'text-figure font-extrabold tracking-tight leading-none tabular-nums sm:text-6xl',
  /* Guide: the small caps-like name over an iOS-style group of rows. */
  groupLabel: 'text-meta font-semibold tracking-wide text-content-tertiary uppercase',
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
