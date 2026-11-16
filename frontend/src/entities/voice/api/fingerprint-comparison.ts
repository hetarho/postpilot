import {
  originFieldKindFromProto,
  type ProtoFingerprintFacetValue,
  type ProtoFingerprintItemComparison,
  type ProtoVoiceHabitPhrase,
} from '@/shared/api'
import type { OriginFieldLocator } from '@/entities/post/@x/voice'
import type {
  FingerprintComparisonItem,
  FingerprintFacet,
  VoiceHabitPhrase,
} from '../model/fingerprint'
import {
  requireFacetUnit,
  requireFingerprintItem,
  requireFingerprintVerdict,
  requireHabitKind,
} from './voice-enums'

const numberOf = (value: ProtoFingerprintFacetValue | undefined) =>
  value?.value.case === 'number' ? value.value.value : 0
const termsOf = (value: ProtoFingerprintFacetValue | undefined) =>
  value?.value.case === 'terms' ? [...value.value.value.terms] : []

/** The fingerprint comparison off the wire, in the server's order (VOICE-62). An item or unit
 *  this build does not know fails the read (ARCH-3). */
export function toComparisons(
  items: ProtoFingerprintItemComparison[],
): FingerprintComparisonItem[] {
  return items.map((item) => ({
    item: requireFingerprintItem(item.item),
    unknown: item.unknown,
    distance: item.distance,
    headline: item.headline,
    verdict: requireFingerprintVerdict(item.verdict),
    facets: item.facets.map((facet): FingerprintFacet => {
      const unit = requireFacetUnit(facet.unit)
      return unit === 'text'
        ? { key: facet.key, unit, voice: termsOf(facet.voice), text: termsOf(facet.text) }
        : { key: facet.key, unit, voice: numberOf(facet.voice), text: numberOf(facet.text) }
    }),
  }))
}

/** The habit phrases off the wire (VOICE-77). A phrase whose field this build cannot name fails
 *  the read rather than underlining some other field (ARCH-3). */
export function toHabits(habits: ProtoVoiceHabitPhrase[]): VoiceHabitPhrase[] {
  return habits.map((habit) => {
    const kind = habit.field ? originFieldKindFromProto(habit.field.kind) : undefined
    if (!kind) throw new Error(`unsupported voice habit field: ${String(habit.field?.kind)}`)
    const field = {
      kind,
      ...(habit.field?.tagIndex !== undefined ? { tagIndex: habit.field.tagIndex } : {}),
      ...(habit.field?.blockIndex !== undefined ? { blockIndex: habit.field.blockIndex } : {}),
      ...(habit.field?.itemIndex !== undefined ? { itemIndex: habit.field.itemIndex } : {}),
    } as OriginFieldLocator
    return {
      field,
      start: habit.start,
      end: habit.end,
      quote: habit.quote,
      kind: requireHabitKind(habit.kind),
      term: habit.term,
      voicePerHundred: habit.voicePerHundred,
      textCount: habit.textCount,
      example: habit.example,
      exampleFictional: habit.exampleFictional,
    }
  })
}
