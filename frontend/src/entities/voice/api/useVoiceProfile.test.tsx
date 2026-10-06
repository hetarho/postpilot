import { create } from '@bufbuild/protobuf'
import { act, renderHook, waitFor } from '@testing-library/react'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { GetVoiceProfileResponseSchema, VoiceProfileSchema, VoiceService } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { DEFAULT_FAKE_VOICE } from '@/test/voice'
import { voiceAnalysisQueryKey } from './voice-queries'
import { useVoiceProfile } from './useVoiceProfile'

describe('useVoiceProfile', () => {
  it('awaits fresh confirmed facts when an analysis response was uncertain', async () => {
    let made = false
    const calls: string[] = []
    const transport = createRouterTransport((router) => {
      router.service(VoiceService, {
        getVoiceProfile: (request) => {
          calls.push(request.voiceId)
          return create(GetVoiceProfileResponseSchema, {
            profile: { voice: { id: request.voiceId, made }, made },
          })
        },
      })
    })
    const { result } = renderHook(() => useVoiceProfile('alice', 'owned-voice'), {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await waitFor(() => expect(result.current.profile?.made).toBe(false))
    made = true
    let refreshed
    await act(async () => {
      refreshed = await result.current.refresh()
    })
    expect(refreshed).toMatchObject({ voice: { id: 'owned-voice' }, made: true })
    expect(calls).toEqual(['owned-voice', 'owned-voice'])
  })

  it('refuses missing identities and unrelated or failed refresh responses', async () => {
    let response: 'owned' | 'foreign' | 'failed' = 'owned'
    let reads = 0
    const transport = createRouterTransport((router) => {
      router.service(VoiceService, {
        getVoiceProfile: (request) => {
          reads++
          if (response === 'failed') throw new ConnectError('Unavailable', Code.Unavailable)
          return create(GetVoiceProfileResponseSchema, {
            profile: { voice: { id: response === 'foreign' ? 'other-voice' : request.voiceId } },
          })
        },
      })
    })
    const { result } = renderHook(() => useVoiceProfile('alice', 'owned-voice'), {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await waitFor(() => expect(result.current.profile?.voice.id).toBe('owned-voice'))
    response = 'foreign'
    await act(async () => {
      await expect(result.current.refresh()).rejects.toThrow('not confirmed')
    })
    response = 'failed'
    await act(async () => {
      await expect(result.current.refresh()).rejects.toMatchObject({ code: Code.Unavailable })
    })
    const empty = renderHook(() => useVoiceProfile('', ''), {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    const before = reads
    await expect(empty.result.current.refresh()).rejects.toThrow('owned writing voice')
    expect(reads).toBe(before)
  })

  it('partitions cached profiles by session owner and by voice', async () => {
    const calls: string[] = []
    const transport = createFakeAuthTransport({
      calls,
      user: { id: 'bob' },
      voice: { activeJobId: 'bob-job' },
    })
    const queryClient = createTestQueryClient()
    const seeded = [
      [voiceAnalysisQueryKey(transport, 'alice', DEFAULT_FAKE_VOICE.id), 'alice-job'],
      [voiceAnalysisQueryKey(transport, 'bob', 'voice-review'), 'bob-review-job'],
    ] as const
    for (const [key, activeJobId] of seeded) {
      queryClient.setQueryData(
        key,
        create(GetVoiceProfileResponseSchema, {
          profile: create(VoiceProfileSchema, { activeJobId }),
        }),
      )
    }

    const { result } = renderHook(() => useVoiceProfile('bob', DEFAULT_FAKE_VOICE.id), {
      wrapper: withProviders(transport, queryClient),
    })

    await waitFor(() => expect(result.current.profile?.activeJobId).toBe('bob-job'))
    expect(result.current.profile?.voice.id).toBe(DEFAULT_FAKE_VOICE.id)
    expect(calls).toContain('GetVoiceProfile')
    // Neither the other account's entry nor the same account's other voice was touched.
    for (const [key, activeJobId] of seeded) {
      expect(queryClient.getQueryData(key)).toMatchObject({ profile: { activeJobId } })
    }
  })
})
