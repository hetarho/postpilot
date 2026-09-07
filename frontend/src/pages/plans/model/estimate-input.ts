import { useEffect, useState } from 'react'
import { ESTIMATOR_COMBOS, type EstimatorCombo, type EstimatorComboName } from '@/entities/plan'
import {
  PLAN_ESTIMATE_BOUNDS,
  PLAN_ESTIMATE_DEFAULTS,
  PLAN_ESTIMATE_STORAGE_KEY,
} from '@/shared/config'

/** What the reader says they will do. It is the whole input to the estimate (QUOTA-41). */
export interface EstimateInput {
  chars: number
  photos: number
  videos: number
}

/** The reader's own case, kept for the browser rather than for the account.
 *
 *  Coming back to `/plans` to compare again should not mean setting three sliders again, and
 *  the case is a question the reader asked rather than a fact about their account — so it
 *  lives in `localStorage` beside the theme, with the same discipline: every access is
 *  guarded, and anything unreadable is treated as absent (THEME-2). */
export function useEstimateInput(): [EstimateInput, (input: EstimateInput) => void] {
  const [input, setInput] = useState<EstimateInput>(readStoredInput)

  useEffect(() => {
    try {
      localStorage.setItem(PLAN_ESTIMATE_STORAGE_KEY, JSON.stringify(input))
    } catch {
      // A private window or a browser told to keep nothing: the session's own value still
      // works, and there is nothing to tell the reader about.
    }
  }, [input])

  return [input, setInput]
}

function readStoredInput(): EstimateInput {
  try {
    const raw = localStorage.getItem(PLAN_ESTIMATE_STORAGE_KEY)
    if (raw === null) return PLAN_ESTIMATE_DEFAULTS
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return PLAN_ESTIMATE_DEFAULTS
    const stored = parsed as Partial<Record<keyof EstimateInput, unknown>>
    return {
      chars: bounded(stored.chars, 'chars'),
      photos: bounded(stored.photos, 'photos'),
      videos: bounded(stored.videos, 'videos'),
    }
  } catch {
    return PLAN_ESTIMATE_DEFAULTS
  }
}

/** A stored value is clamped to the same bounds the sliders offer: the ceilings are the
 *  product's own (a post holds at most 30 photos and 3 clips), so a value from an older
 *  build must not put the estimate outside what a post can even contain. */
function bounded(value: unknown, key: keyof EstimateInput): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return PLAN_ESTIMATE_DEFAULTS[key]
  const { min, max } = PLAN_ESTIMATE_BOUNDS[key]
  return Math.min(max, Math.max(min, Math.round(value)))
}

/** The combo to start on: the reader's last choice if it is still assigned, otherwise the
 *  first one the server published. */
export function firstCombo(combos: readonly EstimatorCombo[]): EstimatorComboName | undefined {
  for (const name of ESTIMATOR_COMBOS) {
    if (combos.some((combo) => combo.combo === name)) return name
  }
  return undefined
}
