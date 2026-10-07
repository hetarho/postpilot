import { useMemo } from 'react'
import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey, useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { WritingTestService as Service } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  writingTestBusy,
  type TestList,
  type TestListInput,
  type TestPublicationResult,
  type WritingTest,
  type WritingTestClient,
} from '../model/types'
import { createWritingTestClient } from './client'

export function writingTestQueryKey(transport: Transport, ownerId: string, id: string) {
  return [
    ...createConnectQueryKey({
      schema: Service.method.getWritingTest,
      input: { testId: id },
      transport,
      cardinality: 'finite',
    }),
    ownerId,
  ] as const
}
export function writingTestsQueryKey(
  transport: Transport,
  ownerId: string,
  input: TestListInput = {},
) {
  return [
    ...createConnectQueryKey({
      schema: Service.method.listWritingTests,
      input: {
        pageSize: input.pageSize ?? 20,
        pageToken: input.pageToken ?? '',
        sourcePostSlug: input.sourcePostSlug ?? '',
        voiceId: input.voiceId ?? '',
      },
      transport,
      cardinality: 'finite',
    }),
    ownerId,
  ] as const
}
export function useWritingTestClient(): WritingTestClient {
  const transport = useTransport()
  return useMemo(() => createWritingTestClient(transport), [transport])
}
export function useWritingTest(ownerId: string, id: string) {
  const transport = useTransport()
  const client = useWritingTestClient()
  const cache = useQueryClient()
  const key = writingTestQueryKey(transport, ownerId, id)
  const query = useQuery({
    queryKey: key,
    queryFn: async ({ signal }) => {
      const incoming = await client.get(id, signal)
      const previous = cache.getQueryData<WritingTest>(key)
      return previous && previous.revision > incoming.revision ? previous : incoming
    },
    enabled: !!ownerId && !!id,
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => (writingTestBusy(query.state.data) ? POLL_INTERVAL_MS : false),
  })
  return { ...query, queryKey: key }
}
export function useWritingTests(ownerId: string, input: TestListInput = {}) {
  const transport = useTransport()
  const client = useWritingTestClient()
  const cache = useQueryClient()
  return useQuery({
    queryKey: writingTestsQueryKey(transport, ownerId, input),
    queryFn: async ({ signal }): Promise<TestList> => {
      const incoming = await client.list(input, signal)
      return {
        ...incoming,
        tests: incoming.tests.map((test) => {
          const previous = cache.getQueryData<WritingTest>(
            writingTestQueryKey(transport, ownerId, test.id),
          )
          return previous && previous.revision > test.revision ? previous : test
        }),
      }
    },
    enabled: !!ownerId,
    retry: false,
    refetchOnWindowFocus: false,
  })
}
function mutationTest(result: WritingTest | TestPublicationResult): WritingTest {
  return 'test' in result ? result.test : result
}
/** Explicit mutation; reads never start, decide, retry or cancel paid work. */
export function useWritingTestMutation<
  K extends 'start' | 'retry' | 'decide' | 'cancel' | 'saveWinner' | 'applyOutput',
>(ownerId: string, method: K) {
  const transport = useTransport()
  const client = useWritingTestClient()
  const cache = useQueryClient()
  type Input = Parameters<WritingTestClient[K]>[0]
  type Output = Awaited<ReturnType<WritingTestClient[K]>>
  return useMutation({
    mutationKey: ['writing-test', ownerId, method],
    retry: false,
    onMutate: () => ({ ownerId, transport }),
    mutationFn: async (input: Input): Promise<Output> => {
      if (!ownerId) throw new Error('Writing test owner unavailable')
      const call = client[method] as (input: Input) => Promise<Output>
      return call(input)
    },
    onSuccess: (result, _input, scope) => {
      const test = mutationTest(result)
      if (!scope?.ownerId) return
      cache.setQueryData<WritingTest>(
        writingTestQueryKey(scope.transport, scope.ownerId, test.id),
        (previous) => (previous && previous.revision > test.revision ? previous : test),
      )
    },
  })
}
