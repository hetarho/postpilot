import { useEffect, useState } from 'react'

/** One clock for the preparation verdict and its source summary, refreshed at expiry/resume. */
export function usePreparationClock(expiries: readonly (string | undefined)[]) {
  const [now, setNow] = useState(Date.now)
  const next = Math.min(
    ...expiries.map((value) => Date.parse(value ?? '')).filter((at) => at > now),
  )
  useEffect(() => {
    const refresh = () => setNow(Date.now())
    const delay = next - Date.now()
    const timer =
      Number.isFinite(delay) && delay <= 2 ** 31 - 1
        ? window.setTimeout(refresh, Math.max(0, delay))
        : undefined
    window.addEventListener('focus', refresh)
    document.addEventListener('visibilitychange', refresh)
    return () => {
      window.clearTimeout(timer)
      window.removeEventListener('focus', refresh)
      document.removeEventListener('visibilitychange', refresh)
    }
  }, [next])
  return now
}
