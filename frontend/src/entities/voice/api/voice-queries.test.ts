import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { VoiceRefSchema, VoiceSchema } from '@/shared/api'
import { toVoice, toVoiceRef } from './voice-queries'

describe('toVoiceRef', () => {
  it('names the voice and whether it is made', () => {
    const ref = create(VoiceRefSchema, { id: 'voice-a', name: '일상', made: true })

    expect(toVoiceRef(ref)).toEqual({ id: 'voice-a', name: '일상', deleted: false, made: true })
  })

  // POST-25: 말투 없음 is an unset message, not an empty one, and reads as no voice at all.
  it('reads an unset reference as 말투 없음', () => {
    expect(toVoiceRef(undefined)).toBeUndefined()
  })
})

// VOICE-52: a directory row carries its 학습 글 count and its analysis date for the meta line.
describe('toVoice', () => {
  it('carries the meta line', () => {
    const voice = toVoice(
      create(VoiceSchema, {
        id: 'voice-a',
        name: '일상',
        made: true,
        materialCount: 3,
        analyzedAt: '2026-09-29T00:00:00Z',
      }),
    )

    expect(voice).toMatchObject({
      made: true,
      materialCount: 3,
      analyzedAt: '2026-09-29T00:00:00Z',
    })
  })
})
