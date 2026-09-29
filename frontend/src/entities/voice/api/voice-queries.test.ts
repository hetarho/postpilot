import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { contentLanguageToProto, VoiceRefSchema } from '@/shared/api'
import { toVoiceRef } from './voice-queries'

describe('toVoiceRef', () => {
  it('keeps a concrete source language', () => {
    const ref = create(VoiceRefSchema, {
      id: 'voice-en',
      made: true,
      sourceLanguage: contentLanguageToProto('en'),
    })

    expect(toVoiceRef(ref)).toMatchObject({ sourceLanguage: 'en', made: true })
  })

  // POST-25: 말투 없음 is an unset message, not an empty one, and reads as no voice at all.
  it('reads an unset reference as 말투 없음', () => {
    expect(toVoiceRef(undefined)).toBeUndefined()
  })

  it('fails closed when an existing voice reference has no source-language provenance', () => {
    expect(() => toVoiceRef(create(VoiceRefSchema, { id: 'voice-legacy' }))).toThrow(
      'unsupported content language enum',
    )
  })
})
