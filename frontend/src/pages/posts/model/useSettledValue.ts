import { useEffect, useState } from 'react'

/** `value` once it has stopped changing for `delayMs`. The list's search follows the URL, which
 *  follows every keystroke (POST-67); the request waits for the typing to stop (POST-91). A value
 *  that goes back to what already settled settles at once. */
export function useSettledValue<T>(value: T, delayMs: number): T {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    if (Object.is(value, settled)) return
    const timer = setTimeout(() => setSettled(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, settled, delayMs])
  return settled
}
