/** The origin every candidate path is resolved against. Any host works — only whether
 *  the value escapes it matters. */
const PROBE_ORIGIN = 'https://postpilot.invalid'

/** True only for a path that stays inside this app.
 *
 *  The router accepts a plain `string` for `to` without checking it against the known
 *  routes, so an attacker-supplied `?redirect=` would otherwise be an open redirect.
 *
 *  The test is delegated to the URL parser rather than written as a pattern, because the
 *  patterns people reach for miss cases the parser knows about: `//evil.com` is
 *  scheme-relative, and `/\evil.com` is too, since the parser folds a backslash into a
 *  slash for http(s) URLs. Anything that resolves to another origin is refused. */
export function isInAppPath(value: string | undefined): value is string {
  if (typeof value !== 'string' || !value.startsWith('/')) return false
  try {
    return new URL(value, PROBE_ORIGIN).origin === PROBE_ORIGIN
  } catch {
    return false
  }
}
