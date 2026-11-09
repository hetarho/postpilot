import { useCallback, useEffect, useRef, useState } from 'react'

type Removal = { key: string; index: number }

/** Preserve the keyboard position when a requested row actually leaves a list. The caller owns
 *  the row controls and the empty-list action; this hook owns no mutation or product identity.
 *  Request only after success, or cancel the returned intent if an optimistic operation fails. */
export function useListRemovalFocus({
  keys,
  getTarget,
  getFallback,
}: {
  keys: readonly string[]
  getTarget: (key: string) => HTMLElement | null | undefined
  getFallback: () => HTMLElement | null | undefined
}) {
  const [pending, setPending] = useState<Removal | null>(null)
  const current = useRef<Removal | null>(null)

  const requestRemoval = useCallback((key: string, index: number) => {
    const request = { key, index }
    current.current = request
    setPending(request)
    return () => {
      // A late refusal of an older operation must not cancel a newer request.
      if (current.current !== request) return
      current.current = null
      setPending(null)
    }
  }, [])

  useEffect(() => {
    if (!pending || current.current !== pending || keys.includes(pending.key)) return
    let canceled = false
    // A fast reread can remove the row in the commit that closes its confirmation. Let every
    // overlay cleanup in that commit return focus before deciding whether focus was lost.
    queueMicrotask(() => {
      if (canceled || current.current !== pending) return
      // Settle even when focus is elsewhere: an ordinary later reread must never claim it.
      current.current = null
      const active = document.activeElement
      if (active && active !== document.body) return
      const key = keys[Math.min(pending.index, keys.length - 1)]
      const target = key === undefined ? getFallback() : getTarget(key)
      target?.focus({ preventScroll: true })
    })
    return () => {
      canceled = true
    }
  }, [pending, keys, getTarget, getFallback])

  return requestRemoval
}
