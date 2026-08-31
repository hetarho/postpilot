import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { contentLanguageToProto, VoiceRefSchema } from '@/shared/api'
import { toVoiceRef } from './voice-queries'

describe('toVoiceRef', () => {
  it('keeps a concrete source language', () => {
    const ref = create(VoiceRefSchema, {
      id: 'voice-en',
      sourceLanguage: contentLanguageToProto('en'),
    })

    expect(toVoiceRef(ref).sourceLanguage).toBe('en')
  })

  it('fails closed when an existing voice reference has no source-language provenance', () => {
    expect(() => toVoiceRef(create(VoiceRefSchema, { id: 'voice-legacy' }))).toThrow(
      'unsupported content language enum',
    )
  })
})
