import { CLIP_PLAYBACK, CLIP_RATES } from '@/shared/config'
import type { ProtoClipObservations } from '@/shared/api'
import type { ClipObservations } from '../model/observations'

export function toClipObservations(value: ProtoClipObservations): ClipObservations {
  if (value.status === 'empty') return { status: 'empty', sources: [] }
  if (
    value.status !== 'available' ||
    !value.sources.length ||
    value.sources.some((item) => !item.source?.id || !item.source.fingerprint)
  ) {
    return { status: 'unavailable', sources: [] }
  }
  return {
    status: 'available',
    sources: value.sources.map((item) => ({
      source: {
        id: item.source!.id,
        fingerprint: item.source!.fingerprint,
        filename: item.source!.filename,
        durationMs: item.source!.durationMs,
        width: item.source!.width,
        height: item.source!.height,
        // A server that predates the rate set offers 1x and faster only, which
        // is exactly what an unverified cadence earns.
        allowedRatePermille:
          item.source!.allowedRatePermille.length > 0
            ? [...item.source!.allowedRatePermille]
            : CLIP_RATES.filter((rate) => rate >= CLIP_PLAYBACK.unit_permille),
      },
      segments: item.segments.map((segment) => ({
        startMs: segment.startMs,
        endMs: segment.endMs,
        event: segment.event,
        subjects: [...segment.subjects],
        speech: segment.speech,
        quality: segment.quality,
      })),
    })),
  }
}
