/** Human-readable time for a list row.
 *
 *  The server sends fixed-width RFC3339 in UTC (spec/policy/posts.md), so the string is
 *  safe to hand straight to `Date`; the browser is what decides the local rendering. */

const MINUTE_MS = 60_000
const HOUR_MS = 60 * MINUTE_MS
const DAY_MS = 24 * HOUR_MS
const WEEK_MS = 7 * DAY_MS

const ABSOLUTE_DATE = new Intl.DateTimeFormat('ko-KR', { dateStyle: 'medium' })

/** "방금" · "3분 전" · … up to a week, then the date itself.
 *
 *  Returns an empty string for a value that is not a timestamp: a list row must not read
 *  "Invalid Date" because one post carries a malformed field. */
export function formatRelativeTime(iso: string, now: Date = new Date()): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''

  // A device clock a little ahead of the server would otherwise render "-1분 전" for a
  // save that just landed.
  const elapsed = now.getTime() - at.getTime()
  if (elapsed < MINUTE_MS) return '방금'
  if (elapsed < HOUR_MS) return `${Math.floor(elapsed / MINUTE_MS)}분 전`
  if (elapsed < DAY_MS) return `${Math.floor(elapsed / HOUR_MS)}시간 전`
  if (elapsed < WEEK_MS) return `${Math.floor(elapsed / DAY_MS)}일 전`
  return ABSOLUTE_DATE.format(at)
}
