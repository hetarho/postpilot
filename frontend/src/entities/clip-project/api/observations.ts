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
