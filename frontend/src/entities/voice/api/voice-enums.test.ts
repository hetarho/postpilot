import { describe, expect, it } from 'vitest'
import {
  ProtoFingerprintFacetUnit,
  ProtoFingerprintItem,
  ProtoVoiceCheckStatus,
  VoiceAiField,
  VoiceNoticeKind,
  VoicePromptPart,
  VoiceSampleKind,
} from '@/shared/api'
import { FINGERPRINT_ITEMS } from '../model/fingerprint'
import {
  requireAiField,
  requireCheckStatus,
  requireFacetUnit,
  requireFingerprintItem,
  requireNoticeKind,
  requirePromptPart,
  requireSampleKind,
} from './voice-enums'

const wireValues = <T extends number>(enumObject: object) =>
  Object.values(enumObject).filter((value): value is T => typeof value === 'number')

// ARCH-3: every value the wire names but UNSPECIFIED maps, and each to its own value.
describe('the voice enum mirrors', () => {
  it('maps every prompt part the wire names', () => {
    const parts = wireValues<VoicePromptPart>(VoicePromptPart)
      .filter((value) => value !== VoicePromptPart.UNSPECIFIED)
      .map(requirePromptPart)
    expect(parts).toEqual(['opening', 'description', 'closing'])
    expect(() => requirePromptPart(VoicePromptPart.UNSPECIFIED)).toThrow()
  })

  it('maps every sample kind the wire names', () => {
    const kinds = wireValues<VoiceSampleKind>(VoiceSampleKind)
      .filter((value) => value !== VoiceSampleKind.UNSPECIFIED)
      .map(requireSampleKind)
    expect(kinds).toEqual(['post', 'answer'])
    expect(() => requireSampleKind(VoiceSampleKind.UNSPECIFIED)).toThrow()
  })

  it('maps every AI field and notice kind the wire names', () => {
    const fields = wireValues<VoiceAiField>(VoiceAiField)
      .filter((value) => value !== VoiceAiField.UNSPECIFIED)
      .map(requireAiField)
    expect(fields).toEqual(['impression', 'tics', 'signature_phrases'])
    expect(wireValues<VoiceNoticeKind>(VoiceNoticeKind).map(requireNoticeKind)).toEqual([
      'none',
      'added',
      'changed',
    ])
  })

  it('maps every fingerprint item and facet unit the wire names', () => {
    const items = wireValues<ProtoFingerprintItem>(ProtoFingerprintItem)
      .filter((value) => value !== ProtoFingerprintItem.UNSPECIFIED)
      .map(requireFingerprintItem)
    expect(items).toEqual([...FINGERPRINT_ITEMS])
    expect(() => requireFingerprintItem(ProtoFingerprintItem.UNSPECIFIED)).toThrow()
    const units = wireValues<ProtoFingerprintFacetUnit>(ProtoFingerprintFacetUnit)
      .filter((value) => value !== ProtoFingerprintFacetUnit.UNSPECIFIED)
      .map(requireFacetUnit)
    expect(units).toEqual(['share', 'per_hundred', 'chars', 'sentences', 'text'])
    expect(() => requireFacetUnit(ProtoFingerprintFacetUnit.UNSPECIFIED)).toThrow()
  })

  it('maps every check status the wire names', () => {
    const statuses = wireValues<ProtoVoiceCheckStatus>(ProtoVoiceCheckStatus)
      .filter((value) => value !== ProtoVoiceCheckStatus.UNSPECIFIED)
      .map(requireCheckStatus)
    expect(statuses).toEqual(['queued', 'running', 'done', 'failed'])
    expect(() => requireCheckStatus(ProtoVoiceCheckStatus.UNSPECIFIED)).toThrow()
  })
})
