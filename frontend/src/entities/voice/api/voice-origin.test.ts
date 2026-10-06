import { expect, it } from 'vitest'
import { ProtoVoiceOrigin } from '@/shared/api'
import { requireVoiceOrigin } from './voice-enums'
it('covers every generated writing-voice origin and preserves unspecified personal snapshots', () => {
  for (const value of Object.values(ProtoVoiceOrigin).filter(
    (value): value is ProtoVoiceOrigin => typeof value === 'number',
  )) {
    expect(requireVoiceOrigin(value)).toBe(
      value === ProtoVoiceOrigin.SYNTHETIC ? 'synthetic' : 'personal',
    )
  }
  expect(requireVoiceOrigin(undefined)).toBe('personal')
  expect(() => requireVoiceOrigin(999 as ProtoVoiceOrigin)).toThrow(
    'unsupported writing voice origin',
  )
})
