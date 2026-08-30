import { create } from '@bufbuild/protobuf'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { GetVoiceProfileResponseSchema, VoiceProfileSchema } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { DEFAULT_FAKE_VOICE } from '@/test/voice'
import { voiceProfileQueryKey } from './voice-queries'
import { useVoiceProfile } from './useVoiceProfile'

describe('useVoiceProfile', () => {
  it('partitions cached profiles by session owner and by voice', async () => {
    const calls: string[] = []
    const transport = createFakeAuthTransport({
      calls,
      user: { id: 'bob' },
      voice: { styleguide: 'bob style' },
    })
    const queryClient = createTestQueryClient()
    const seeded = [
      [voiceProfileQueryKey(transport, 'alice', DEFAULT_FAKE_VOICE.id), 'alice style'],
      [voiceProfileQueryKey(transport, 'bob', 'voice-review'), 'bob review style'],
    ] as const
    for (const [key, styleguide] of seeded) {
      queryClient.setQueryData(
        key,
        create(GetVoiceProfileResponseSchema, {
          profile: create(VoiceProfileSchema, { styleguide }),
        }),
      )
    }

    const { result } = renderHook(() => useVoiceProfile('bob', DEFAULT_FAKE_VOICE.id), {
      wrapper: withProviders(transport, queryClient),
    })

    await waitFor(() => expect(result.current.profile?.styleguide).toBe('bob style'))
    expect(result.current.profile?.voice.id).toBe(DEFAULT_FAKE_VOICE.id)
    expect(calls).toContain('GetVoiceProfile')
    // Neither the other account's entry nor the same account's other voice was touched.
    for (const [key, styleguide] of seeded) {
      expect(queryClient.getQueryData(key)).toMatchObject({ profile: { styleguide } })
    }
  })
})
