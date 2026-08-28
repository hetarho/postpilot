// A framework-free notification that the session died mid-use.
//
// It exists because the two halves of the problem live in layers that cannot see each
// other: the transport is the only place that observes a 401, and only the app layer
// owns the router that can act on one. shared/api is a pure layer (no react, no
// router), so it publishes the event and lets app/providers subscribe.

export type UnauthenticatedListener = () => void

const listeners = new Set<UnauthenticatedListener>()

/** Subscribes to session-lost events. Returns the unsubscribe function. */
export function onUnauthenticated(listener: UnauthenticatedListener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Notifies every listener. Called by the transport interceptor. */
export function emitUnauthenticated(): void {
  // Iterate a copy: a listener that unsubscribes itself would otherwise mutate the set
  // mid-iteration. A listener that throws must not take down the RPC that triggered it.
  for (const listener of [...listeners]) {
    try {
      listener()
    } catch {
      // Nothing useful to do here — the caller is an RPC, not the app shell.
    }
  }
}
