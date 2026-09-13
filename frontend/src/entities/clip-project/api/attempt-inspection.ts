import type { ProtoClipAttemptInspection } from '@/shared/api'
import type { ClipAttemptInspection } from '../model/types'
import { toClipObservations } from './observations'

export function toClipAttemptInspection(value: ProtoClipAttemptInspection): ClipAttemptInspection {
  const out: ClipAttemptInspection = {
    jobId: value.jobId,
    status: 'unavailable',
    stage: value.stage,
    completedChunks: 0,
    totalChunks: 0,
    completedSources: 0,
    totalSources: 0,
    observations: { status: 'empty', sources: [] },
    ranges: [],
    validationCheck: 'unknown',
    validationPhase: '',
    measurements: {},
  }
  if (value.status === 'missing') return { ...out, status: 'missing' }
  const bounded = (n: number, max: number) => Number.isInteger(n) && n >= 0 && n <= max
  if (
    value.status !== 'available' ||
    !value.jobId ||
    !bounded(value.totalChunks, 49) ||
    !bounded(value.completedChunks, value.totalChunks) ||
    !bounded(value.totalSources, 20) ||
    !bounded(value.completedSources, value.totalSources) ||
    value.ranges.length > 100
  )
    return out
  const observations = value.observations
    ? toClipObservations(value.observations)
    : out.observations
  const ranges = value.ranges.map((r) => {
    const source = observations.sources[r.source - 1]?.source
    return {
      cut: r.cut,
      source: r.source,
      startMs: r.startMs,
      endMs: r.endMs,
      valid:
        r.valid &&
        bounded(r.cut, 100) &&
        r.cut > 0 &&
        !!source &&
        bounded(r.startMs, source.durationMs) &&
        bounded(r.endMs, source.durationMs) &&
        r.endMs > r.startMs,
    }
  })
  const unsigned = new Set([
    'source',
    'chunk',
    'cut',
    'cut_count',
    'target_ms',
    'before_ms',
    'after_ms',
    'remaining_ms',
    'min_ms',
    'max_ms',
    'transition_ms',
    'backward_ms',
    'segment',
    'segment_count',
    'duration_ms',
    'expected_index',
    'event_runes',
    'speech_runes',
    'quality_runes',
    'subject_count',
    'subject_runes',
  ])
  const signed = new Set([
    'actual_index',
    'start_ms',
    'end_ms',
    'previous_end_ms',
    'raw_start_ms',
    'raw_end_ms',
    'focal_x_ppm',
    'focal_y_ppm',
    'subject_x_ppm',
    'subject_y_ppm',
    'subject_width_ppm',
    'subject_height_ppm',
  ])
  return {
    ...out,
    status: 'available',
    observations,
    ranges,
    evidenceLimited: value.evidenceLimited,
    completedChunks: value.completedChunks,
    totalChunks: value.totalChunks,
    completedSources: value.completedSources,
    totalSources: value.totalSources,
    validationCheck: /^[a-z_]{1,64}$/.test(value.validationCheck)
      ? value.validationCheck
      : 'unknown',
    validationPhase: value.validationPhase,
    measurements: Object.fromEntries(
      Object.entries(value.measurements).filter(([key, n]) =>
        unsigned.has(key)
          ? bounded(n, 180000000)
          : signed.has(key) && Number.isInteger(n) && Math.abs(n) <= 180000000,
      ),
    ),
  }
}
