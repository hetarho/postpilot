import { extendTailwindMerge } from 'tailwind-merge'

/** The Guide's own type steps, registered as `--text-*` tokens in app/styles/index.css (THEME-11
 *  →THEME-19). Tailwind's stock scale supplies the rest (12 `xs`, 18 `lg`, 24 `2xl`, 30/36
 *  `3xl`/`4xl`, 60 `6xl`). Kept in step with the `@theme` block by app/styles/type-steps.test.ts. */
export const TYPE_STEPS = [
  'meta',
  'label',
  'body',
  'title',
  'step-title',
  'step-title-wide',
  'large-title',
  'large-title-wide',
  'figure',
] as const

/** The one `twMerge` the frontend uses. tailwind-merge only knows Tailwind's stock t-shirt sizes,
 *  so it read `text-meta` as a COLOUR: a caller's `text-content-secondary` then erased the size
 *  of every recipe it was merged into. Told the Guide's steps, it keeps a size and a colour apart. */
export const twMerge = extendTailwindMerge({ extend: { theme: { text: [...TYPE_STEPS] } } })
