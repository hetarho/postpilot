import { useSyncExternalStore } from 'react'

const subscribe = (notify: () => void) => {
  window.addEventListener('resize', notify)
  window.visualViewport?.addEventListener('resize', notify)
  window.visualViewport?.addEventListener('scroll', notify)
  return () => {
    window.removeEventListener('resize', notify)
    window.visualViewport?.removeEventListener('resize', notify)
    window.visualViewport?.removeEventListener('scroll', notify)
  }
}
export function useVisualViewport() {
  const height = useSyncExternalStore(
    subscribe,
    () => window.visualViewport?.height ?? window.innerHeight,
  )
  const offsetTop = useSyncExternalStore(subscribe, () => window.visualViewport?.offsetTop ?? 0)
  return { height, offsetTop }
}
