import { describe, expect, it } from 'vitest'
import { VoicePromptPart, VoiceSampleKind } from '@/shared/api'
import { requirePromptPart, requireSampleKind } from './voice-enums'

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
})
