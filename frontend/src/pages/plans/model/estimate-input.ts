import { useEffect, useState } from 'react'
import { CLIP_ESTIMATE_BOUNDS, CLIP_ESTIMATE_DEFAULTS, CLIP_ESTIMATE_STORAGE_KEY } from '../config'

export type EstimateKind = 'blog' | 'clip'
export interface ClipEstimateInput {
  sources: number
  seconds: number
}

type Bounds<T> = { [K in keyof T]: { min: number; max: number } }

/** A browser preference for the clip conditions.
 * Every read/write is guarded; stale or edited values cannot exceed product bounds. */
function useStoredInput<T extends { [K in keyof T]: number }>(
  key: string,
  defaults: T,
  bounds: Bounds<T>,
): [T, (input: T) => void] {
  const [input, setInput] = useState<T>(() => {
    try {
      const parsed: unknown = JSON.parse(localStorage.getItem(key) ?? 'null')
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return defaults
      const stored = parsed as Record<keyof T, unknown>
      const result = { ...defaults }
      for (const field of Object.keys(defaults) as (keyof T)[]) {
        const value = stored[field]
        if (typeof value === 'number' && Number.isFinite(value)) {
          const { min, max } = bounds[field]
          result[field] = Math.min(max, Math.max(min, Math.round(value))) as T[keyof T]
        }
      }
      return result
    } catch {
      return defaults
    }
  })
  useEffect(() => {
    try {
      localStorage.setItem(key, JSON.stringify(input))
    } catch {
      /* Session still works. */
    }
  }, [input, key])
  return [input, setInput]
}

export function useClipEstimateInput(): [ClipEstimateInput, (input: ClipEstimateInput) => void] {
  return useStoredInput<ClipEstimateInput>(
    CLIP_ESTIMATE_STORAGE_KEY,
    CLIP_ESTIMATE_DEFAULTS,
    CLIP_ESTIMATE_BOUNDS,
  )
}
