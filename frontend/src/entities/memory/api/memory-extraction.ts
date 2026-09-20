import { useMemo, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { MemoryService } from '@/shared/api'
import type { MemoryKind } from '../model/types'
import { invalidateMemories } from './memory-cache'
import { memoryErrorMessage } from './memory-errors'
import { toMemoryCandidate, toProtoKind, type MemoryCandidate } from './memory-queries'

/** 기억으로 저장 (MEM-13): one durable, credit-gated job. The refusal an account without the
 *  balance meets is the queue's own, and it arrives here as an ordinary Connect error. */
export function useStartMemoryExtraction() {
  const mutation = useMutation(MemoryService.method.startMemoryExtraction)
  return {
    ...mutation,
    errorMessage: memoryErrorMessage(mutation.error),
    start: (postSlug: string) => mutation.mutateAsync({ postSlug }),
  }
}

/** The candidates of a FINISHED extraction. Enabled only once the job is done: while it runs the
 *  row still carries its input, and the server answers `MEMORY_EXTRACTION_NOT_READY` rather than
 *  an empty list — because "this post yielded nothing" is a real answer the user acts on. */
export function useMemoryExtraction(jobId: string, ready: boolean) {
  const query = useQuery(
    MemoryService.method.getMemoryExtraction,
    { jobId },
    { enabled: jobId !== '' && ready, staleTime: 0 },
  )
  // Memoized on the answer, not rebuilt per render: the surface seeds its checkbox state from
  // this list, and a new array identity every render would re-seed it forever.
  const candidates = useMemo(
    () => (query.data?.candidates ?? []).map(toMemoryCandidate),
    [query.data],
  )
  return {
    postSlug: query.data?.postSlug ?? '',
    candidates,
    isPending: query.isPending,
    isError: query.isError,
    errorMessage: memoryErrorMessage(query.error),
  }
}

export interface CandidateSaveOutcome {
  saved: number
  /** One message per candidate that could not be saved, by its index in the list the user
   *  checked. The rest are kept: a refusal on one row is not a reason to lose the others. */
  failures: Array<{ index: number; message: string }>
}

/** Approving checked candidates, one create each (MEM-15). The same shape 지침's 전부 수락 uses:
 *  a per-row refusal (a duplicate text, the account cap, a bound) stays with its row and the walk
 *  continues, because the alternative is losing every approval to one bad row. */
export function useApproveMemoryCandidates(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const [running, setRunning] = useState(false)

  const approve = async (
    postSlug: string,
    candidates: readonly { index: number; text: string; kind: MemoryKind; tags: string[] }[],
  ): Promise<CandidateSaveOutcome> => {
    const client = createClient(MemoryService, transport)
    const failures: CandidateSaveOutcome['failures'] = []
    let saved = 0
    setRunning(true)
    try {
      for (const candidate of candidates) {
        try {
          await client.createMemory({
            text: candidate.text.trim(),
            kind: toProtoKind(candidate.kind),
            tags: candidate.tags,
            // The post it was approved from, so the memory outlives that post only while it has
            // another source (MEM-17).
            sourcePostSlug: postSlug,
          })
          saved += 1
        } catch (cause) {
          failures.push({ index: candidate.index, message: memoryErrorMessage(cause) })
        }
      }
    } finally {
      setRunning(false)
      if (saved > 0) invalidateMemories(queryClient, transport, ownerId)
    }
    return { saved, failures }
  }

  return { approve, isPending: running }
}

export type { MemoryCandidate }
