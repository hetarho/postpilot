import type { AnyRouter } from '@tanstack/react-router'
import { TAB_DESTINATIONS } from './navigation'

/** The kit screens 홈's rings open (the writing AI's is AI 토너먼트, a tab). Named here rather than
 *  read from the writing-kit widget, whose barrel would pull 홈's whole picture into the shell. */
const KIT_SCREENS = ['/voices', '/templates', '/guidelines', '/memories'] as const

/** The screens one tap away from wherever the owner is: the four tabs and the kit screens. Their
 *  code is fetched while the app is idle, so a first visit to one swaps in as a second one does
 *  instead of waiting on its chunks with the tap seemingly unheard. */
export const IDLE_PRELOAD_PATHS: readonly string[] = [
  ...TAB_DESTINATIONS.map((destination) => destination.to),
  ...KIT_SCREENS,
]

/** The screen in view asks for its own data first; the next screens' code waits this long. */
const FIRST_DELAY_MS = 1000
const IDLE_TIMEOUT_MS = 4000
/** Where the browser has no idle callback (Safari), a short wait stands in. */
const FALLBACK_DELAY_MS = 300

function whenIdle(work: () => void): () => void {
  if (typeof window.requestIdleCallback === 'function') {
    const handle = window.requestIdleCallback(work, { timeout: IDLE_TIMEOUT_MS })
    return () => window.cancelIdleCallback(handle)
  }
  const handle = window.setTimeout(work, FALLBACK_DELAY_MS)
  return () => window.clearTimeout(handle)
}

/** A connection that asks to save data keeps to what is tapped; intent preloading still runs. */
function savesData(): boolean {
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection
  return connection?.saveData === true
}

/** Preloads `paths` one at a time, each in an idle moment of its own, skipping `current`, from a
 *  moment after the screen in view has started. Only the route's code and its guards run — the
 *  same work an intent preload does — never a page's data. Returns the way to stop. */
export function preloadWhenIdle(
  router: AnyRouter,
  current: string,
  paths: readonly string[] = IDLE_PRELOAD_PATHS,
): () => void {
  const queue = paths.filter((path) => path !== current)
  if (savesData() || queue.length === 0) return () => {}
  let stopped = false
  let cancel = () => {}
  const next = (index: number) => {
    if (stopped || index >= queue.length) return
    cancel = whenIdle(() => {
      void router
        .preloadRoute({ to: queue[index] })
        .catch(() => undefined)
        .finally(() => next(index + 1))
    })
  }
  const first = window.setTimeout(() => next(0), FIRST_DELAY_MS)
  return () => {
    stopped = true
    window.clearTimeout(first)
    cancel()
  }
}
