import { act, renderHook } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import { appFailureFromConnect } from '@/shared/api'
import { useStartModelExperiment } from './useStartModelExperiment'
import { useStartWriteExperiment } from './useStartWriteExperiment'
import { useStartVoiceReflection } from './useStartVoiceReflection'
import { useExperimentActions } from './useExperimentActions'

const useMutation = vi.hoisted(() => vi.fn())
vi.mock('@connectrpc/connect-query', () => ({ useMutation }))
const a = { providerId: 'p', modelId: 'a' }
const b = { providerId: 'p', modelId: 'b' }
beforeEach(() => useMutation.mockClear())

it.each(['observe', 'write', 'voice'] as const)(
  'refuses the retired %s start locally with a common-test reason',
  async (kind) => {
    const { result } = renderHook(() => ({
      observe: useStartModelExperiment(),
      write: useStartWriteExperiment(),
      voice: useStartVoiceReflection(),
    }))
    let rejection: unknown
    await act(async () => {
      try {
        if (kind === 'observe') await result.current.observe.startObserve('post', a, b)
        if (kind === 'write') await result.current.write.start('post', 'editor', a, a, b)
        if (kind === 'voice') await result.current.voice.start('voice', 'prompt', a, b)
      } catch (error) {
        rejection = error
      }
    })
    expect(appFailureFromConnect(rejection)).toEqual({
      reason: 'WRITING_TEST_LEGACY_READ_ONLY',
      params: {},
    })
    expect(result.current[kind].failure?.reason).toBe('WRITING_TEST_LEGACY_READ_ONLY')
    expect(result.current[kind].isPending).toBe(false)
    expect(result.current[kind].errorMessage).toContain('글쓰기 테스트')
    expect(useMutation).not.toHaveBeenCalled()
  },
)

it('refuses every ranking, vote, retry and follow-up without owner refresh or provider requests', async () => {
  const refresh = vi.fn().mockResolvedValue(undefined)
  const { result } = renderHook(() => useExperimentActions('retained', refresh))
  const operations = [
    () => result.current.complete([]),
    () => result.current.applyCandidate('a'),
    () => result.current.adoptCandidate('a'),
    () => result.current.choose('a'),
    () => result.current.decideWrite('a', true),
    () => result.current.useSingle('a'),
    () => result.current.dismiss(),
    () => result.current.retry(),
    () => result.current.apply(),
    () => result.current.adopt(),
  ]
  await act(async () => {
    for (const operation of operations) {
      await expect(operation()).rejects.toThrow()
    }
  })
  expect(useMutation).not.toHaveBeenCalled()
  expect(refresh).not.toHaveBeenCalled()
  expect(result.current.failure?.reason).toBe('WRITING_TEST_LEGACY_READ_ONLY')
})
