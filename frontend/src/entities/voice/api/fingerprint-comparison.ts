import type { ProtoFingerprintFacetValue, ProtoFingerprintItemComparison } from '@/shared/api'
import type { FingerprintComparisonItem, FingerprintFacet } from '../model/fingerprint'
import { requireFacetUnit, requireFingerprintItem } from './voice-enums'

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
    facets: item.facets.map((facet): FingerprintFacet => {
      const unit = requireFacetUnit(facet.unit)
      return unit === 'text'
        ? { key: facet.key, unit, voice: termsOf(facet.voice), text: termsOf(facet.text) }
        : { key: facet.key, unit, voice: numberOf(facet.voice), text: numberOf(facet.text) }
    }),
  }))
}
