import { ProtoReplacementSurface, type ProtoReplacementCandidate } from '@/shared/api'
import type { ReplacementCandidate, ReplacementSurface } from '../model/replacements'

// Closed both ways (ARCH-3): a number this build does not know is undefined, never a guess.
const TO_PROTO: Record<ReplacementSurface, ProtoReplacementSurface> = {
  title: ProtoReplacementSurface.TITLE,
  tag: ProtoReplacementSurface.TAG,
  body: ProtoReplacementSurface.BODY,
}

const FROM_PROTO = new Map<ProtoReplacementSurface, ReplacementSurface>(
  Object.entries(TO_PROTO).map(([surface, wire]) => [wire, surface as ReplacementSurface]),
)

export function replacementSurfaceFromProto(
  value: ProtoReplacementSurface,
): ReplacementSurface | undefined {
  return FROM_PROTO.get(value)
}

export function replacementSurfaceToProto(surface: ReplacementSurface): ProtoReplacementSurface {
  return TO_PROTO[surface]
}

/** Undefined for a surface this build cannot name: a dropped offer changes nothing (GEN-53). */
export function toReplacementCandidate(
  candidate: ProtoReplacementCandidate,
): ReplacementCandidate | undefined {
  const surface = replacementSurfaceFromProto(candidate.surface)
  if (!surface) return undefined
  return {
    surface,
    index: candidate.index,
    source: candidate.source,
    phrases: [...candidate.phrases],
  }
}
