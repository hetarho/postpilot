import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { WritingVoiceCandidateService as Service } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import {
  latestWritingVoiceCandidatesQueryKey,
  useAdoptWritingVoiceCandidate,
  useLatestWritingVoiceCandidates,
  useWritingVoiceCandidates,
} from './hooks'

const rows = Array.from({ length: 8 }, (_, n) => ({
  id: `${n}`,
  name: 'style',
  description: 'feel',
  sample: 'fictional',
}))
it('keeps result reads owner- and transport-scoped and rejects incomplete batches', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getWritingVoiceCandidates, (request) =>
      create(Service.method.getWritingVoiceCandidates.output, {
        jobId: request.jobId,
        candidates: request.jobId === 'incomplete' ? rows.slice(0, 7) : rows,
      }),
    )
    rpc(Service.method.getLatestWritingVoiceCandidates, () =>
      create(Service.method.getLatestWritingVoiceCandidates.output, {}),
    )
  })
  const otherTransport = createRouterTransport(() => {})
  expect(latestWritingVoiceCandidatesQueryKey(transport, 'alice')).not.toEqual(
    latestWritingVoiceCandidatesQueryKey(transport, 'bob'),
  )
  expect(latestWritingVoiceCandidatesQueryKey(transport, 'alice')).not.toEqual(
    latestWritingVoiceCandidatesQueryKey(otherTransport, 'alice'),
  )
  const cache = createTestQueryClient()
  const valid = renderHook(() => useWritingVoiceCandidates('alice', 'valid'), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(valid.result.current.candidates).toHaveLength(8))
  const invalid = renderHook(() => useWritingVoiceCandidates('bob', 'incomplete'), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(invalid.result.current.isError).toBe(true))
  expect(invalid.result.current.candidates).toHaveLength(0)
})

it('invalidates only the adopting owner and preserves the confirmed server default state', async () => {
  let reads = 0
  let adoptions = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getLatestWritingVoiceCandidates, () => {
      reads++
      return create(Service.method.getLatestWritingVoiceCandidates.output, {
        jobId: 'job',
        resultJobId: 'job',
        candidates: rows,
      })
    })
    rpc(Service.method.adoptWritingVoiceCandidate, (request) => {
      expect(request.makeDefault).toBe(true)
      adoptions++
      return create(Service.method.adoptWritingVoiceCandidate.output, {
        voice: {
          id: 'already-adopted',
          name: 'Renamed by the owner',
          made: true,
          isDefault: false,
          origin: 2,
        },
      })
    })
  })
  const cache = createTestQueryClient()
  cache.setQueryData(
    latestWritingVoiceCandidatesQueryKey(transport, 'bob'),
    create(Service.method.getLatestWritingVoiceCandidates.output, {}),
  )
  const view = renderHook(
    () => ({
      latest: useLatestWritingVoiceCandidates('alice'),
      adopt: useAdoptWritingVoiceCandidate('alice'),
    }),
    {
      wrapper: withProviders(transport, cache),
    },
  )
  await waitFor(() => expect(view.result.current.latest.batch?.candidates).toHaveLength(8))
  let voice
  await act(async () => {
    voice = await view.result.current.adopt.adopt({ jobId: 'job', candidateId: '0' })
  })
  expect(voice).toMatchObject({
    id: 'already-adopted',
    isDefault: false,
    name: 'Renamed by the owner',
  })
  await waitFor(() => expect(reads).toBe(2))
  expect(
    cache.getQueryState(latestWritingVoiceCandidatesQueryKey(transport, 'bob'))?.isInvalidated,
  ).toBe(false)
  expect(adoptions).toBe(1)
})

it('rejects an adoption without a confirmed made voice', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.adoptWritingVoiceCandidate, () =>
      create(Service.method.adoptWritingVoiceCandidate.output, {
        voice: { id: 'unfinished', made: false },
      }),
    )
  })
  const view = renderHook(() => useAdoptWritingVoiceCandidate('alice'), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await expect(view.result.current.adopt({ jobId: 'job', candidateId: '0' })).rejects.toThrow(
    'saved writing voice unavailable',
  )
})
