import type { ClipRenderKind } from './types'

/** Supported local execution leads independently of the historical result kind. */
export function preferredClipRenderKind(
  _lastKind: ClipRenderKind | undefined,
  browserAvailable: boolean,
): ClipRenderKind {
  return browserAvailable ? 'browser' : 'server'
}
