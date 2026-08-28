import { create } from '@bufbuild/protobuf'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { GetVoiceProfileResponseSchema, VoiceProfileSchema } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { voiceProfileQueryKey } from './voice-queries'
import { useVoiceProfile } from './useVoiceProfile'

describe('useVoiceProfile', () => {
  it('partitions cached profiles by session owner', async () => {
    const calls: string[] = []
    const transport = createFakeAuthTransport({
      calls,
      user: { id: 'bob' },
      voice: { styleguide: 'bob style' },
    })
    const queryClient = createTestQueryClient()
    queryClient.setQueryData(
      voiceProfileQueryKey(transport, 'alice'),
      create(GetVoiceProfileResponseSchema, {
        profile: create(VoiceProfileSchema, { styleguide: 'alice style' }),
      }),
    )

    const { result } = renderHook(() => useVoiceProfile('bob'), {
      wrapper: withProviders(transport, queryClient),
    })

    await waitFor(() => expect(result.current.profile?.styleguide).toBe('bob style'))
    expect(calls).toContain('GetVoiceProfile')
    expect(queryClient.getQueryData(voiceProfileQueryKey(transport, 'alice'))).toMatchObject({
      profile: { styleguide: 'alice style' },
    })
  })
})
