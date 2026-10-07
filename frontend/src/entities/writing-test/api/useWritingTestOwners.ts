import { useCallback } from 'react'
import { useTransport, createConnectQueryKey } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidateWrittenPost } from '@/entities/post/@x/writing-test'
import { getSelectionsQueryKey } from '@/entities/model-catalog/@x/writing-test'
import { WritingTestService } from '@/shared/api'
import type { WritingTest, WritingTestPublication } from '../model/types'

/** Refresh the same owner's operational records and canonical destination only after
 * an authoritative receipt. Reading a champion neither publishes nor refreshes targets. */
export function useWritingTestOwnerRefresh(ownerId: string) {
  const transport = useTransport(),
    cache = useQueryClient()
  return useCallback(
    async (test: WritingTest, publication: WritingTestPublication) => {
      if (
        !ownerId ||
        publication.status !== 'confirmed' ||
        publication.testId !== test.id ||
        publication.winnerCandidateId !== test.winnerCandidateId
      )
        return
      const historyKey = [
        ...createConnectQueryKey({
          schema: WritingTestService.method.listWritingTests,
          transport,
          cardinality: 'finite',
        }),
        ownerId,
      ]
      await Promise.all([
        cache.invalidateQueries({ queryKey: historyKey }),
        ...(publication.action === 'apply-output' && test.sourcePostSlug
          ? [invalidateWrittenPost(cache, transport, test.sourcePostSlug)]
          : []),
        ...(publication.action === 'adopt-model'
          ? [cache.invalidateQueries({ queryKey: getSelectionsQueryKey(transport) })]
          : []),
      ])
    },
    [ownerId, transport, cache],
  )
}
