import { useEffect, useEffectEvent } from 'react'
import type { AuthoringKind } from '@/entities/ai-authoring'
import { REMOVAL_FOCUS_HANDOFF_TTL_MS } from '../config/focus'

/** A template is deleted on its own reading, which then leaves for its directory (CLIP-42), so the
 *  delete button that held keyboard focus is gone with the page. The reading hands focus over here
 *  and the directory takes it once its list is ready (THEME-76), instead of focus falling to the
 *  document body. One handoff at a time, consumed by the next matching directory mount. */
let pending: { kind: AuthoringKind; at: number } | null = null

export function handOffRemovalFocus(kind: AuthoringKind) {
  pending = { kind, at: Date.now() }
}

/** Focuses `getTarget()` once `ready` when a removal of `kind` handed focus over recently. */
export function useRemovalFocusHandoff(
  kind: AuthoringKind,
  ready: boolean,
  getTarget: () => HTMLElement | null | undefined,
) {
  const focusTarget = useEffectEvent(() => getTarget()?.focus())
  useEffect(() => {
    if (!ready || !pending || pending.kind !== kind) return
    const fresh = Date.now() - pending.at <= REMOVAL_FOCUS_HANDOFF_TTL_MS
    pending = null
    if (fresh) focusTarget()
  }, [kind, ready])
}
