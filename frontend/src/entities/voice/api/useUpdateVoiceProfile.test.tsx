import { create } from '@bufbuild/protobuf'
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { GetVoiceProfileResponseSchema, VoiceProfileSchema } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { DEFAULT_FAKE_VOICE } from '@/test/voice'
import { voiceProfileQueryKey } from './voice-queries'
import { useUpdateVoiceProfile } from './useUpdateVoiceProfile'

describe('useUpdateVoiceProfile', () => {
  it('patches only rules when a newer analyzed styleguide is already cached', async () => {
    const transport = createFakeAuthTransport({
      user: { id: 'alice' },
      voice: { styleguide: '서버 응답의 이전 문체', rules: '이전 추가 규칙' },
    })
    const queryClient = createTestQueryClient()
    const key = voiceProfileQueryKey(transport, 'alice', DEFAULT_FAKE_VOICE.id)
    queryClient.setQueryData(
      key,
      create(GetVoiceProfileResponseSchema, {
        profile: create(VoiceProfileSchema, {
          styleguide: '분석 완료로 갱신된 문체',
          rules: '이전 추가 규칙',
        }),
      }),
    )
    const { result } = renderHook(() => useUpdateVoiceProfile('alice', DEFAULT_FAKE_VOICE.id), {
      wrapper: withProviders(transport, queryClient),
    })

    await act(() => result.current.saveRules('새 추가 규칙'))

    expect(queryClient.getQueryData(key)).toMatchObject({
      profile: { styleguide: '분석 완료로 갱신된 문체', rules: '새 추가 규칙' },
    })
  })
})
