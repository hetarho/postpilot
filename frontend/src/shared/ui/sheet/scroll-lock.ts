interface BodyLock {
  owners: Set<symbol>
  overflow: string
}

const locks = new WeakMap<HTMLElement, BodyLock>()

/** Body scrolling belongs to all open sheets together, regardless of their teardown order. */
export function lockBodyScroll(body: HTMLElement): () => void {
  let lock = locks.get(body)
  if (!lock) {
    lock = { owners: new Set(), overflow: body.style.overflow }
    locks.set(body, lock)
    body.style.overflow = 'hidden'
  }
  const owner = Symbol()
  lock.owners.add(owner)
  return () => {
    if (!lock.owners.delete(owner) || lock.owners.size > 0) return
    body.style.overflow = lock.overflow
    locks.delete(body)
  }
}
