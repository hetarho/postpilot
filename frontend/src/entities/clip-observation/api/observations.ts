import { CLIP_PLAYBACK, CLIP_RATES } from '@/entities/clip-design/@x/clip-observation'
import type { ProtoClipObservations } from '@/shared/api'
import type {
  ClipObservationCertainty,
  ClipObservations,
  ClipObservationUsability,
} from '../model/observations'

// A status the server did not record — an older record, or an older server —
// reads as unspecified. Nothing here promotes a blank to 'certain'.
const CERTAINTY: ClipObservationCertainty[] = ['certain', 'uncertain', 'unknown']
const USABILITY: ClipObservationUsability[] = ['usable', 'unusable']

function certainty(value: string): ClipObservationCertainty {
  return CERTAINTY.find((known) => known === value) ?? 'unspecified'
}
function usability(value: string): ClipObservationUsability {
  return USABILITY.find((known) => known === value) ?? 'unspecified'
}

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
        ...(segment.focal ? { focal: { x: segment.focal.x, y: segment.focal.y } } : {}),
        startMs: segment.startMs,
        endMs: segment.endMs,
        event: segment.event,
        action: segment.action,
        motion: segment.motion,
        subjects: [...segment.subjects],
        speech: segment.speech,
        quality: segment.quality,
        certainty: certainty(segment.certainty),
        usability: usability(segment.usability),
      })),
    })),
  }
}
