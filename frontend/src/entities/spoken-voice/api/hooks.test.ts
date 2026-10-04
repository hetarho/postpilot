import { hashKey } from '@tanstack/react-query'
import { expect, it } from 'vitest'
import { spokenVoiceFixture } from '@/test/spoken-voices'
import { spokenScope } from './hooks'

it('never reuses a private voice cache across owners or transport connections', () => {
  const first = spokenVoiceFixture().transport
  const second = spokenVoiceFixture().transport
  const alice = hashKey(spokenScope(first, 'alice'))
  expect(hashKey(spokenScope(first, 'alice'))).toBe(alice)
  expect(hashKey(spokenScope(first, 'bob'))).not.toBe(alice)
  expect(hashKey(spokenScope(second, 'alice'))).not.toBe(alice)
})
