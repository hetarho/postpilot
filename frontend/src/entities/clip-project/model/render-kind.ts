import type { ClipRenderKind } from './types'

/** Preference belongs to the last successful result, never to a stored setting. */
export function preferredClipRenderKind(
  lastKind: ClipRenderKind | undefined,
  browserAvailable: boolean,
): ClipRenderKind {
  return browserAvailable ? (lastKind ?? 'browser') : 'server'
}
