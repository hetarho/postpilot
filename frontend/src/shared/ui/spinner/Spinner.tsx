import { twMerge } from 'tailwind-merge'

/** The in-place pending indicator. Exists so `Button` can show a pending state without changing
 *  size (design-language §6) — a label swap moves the target out from under the thumb that just
 *  pressed it, and in a wrapping row it re-flows every neighbour.
 *
 *  `currentColor` rather than a token: the spinner always belongs to the control it sits in, so it
 *  inherits whatever foreground that control's variant already resolved. */
export function Spinner({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={twMerge(
        'size-4 shrink-0 animate-spin rounded-full border-2 border-current border-t-transparent',
        className,
      )}
    />
  )
}
