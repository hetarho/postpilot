import { useMemo, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { MemoryService, appFailureFromProto } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { invalidateMemories } from './memory-cache'
import { memoryErrorMessage } from './memory-errors'
import { toMemoryCandidate, type MemoryCandidate } from './memory-queries'

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

/** The one ruling on an extraction (MEM-15): the checked candidates are created on the server
 *  from the stored list, the rest discarded, and once every checked one is stored the job's
 *  payload is cleared so nothing can read it again (MEM-16). A per-row refusal (the account cap,
 *  a bound) comes back with its index and keeps the extraction open, so a retry sends only what
 *  failed — the ones that saved deduplicate rather than doubling. */
export function useApproveMemoryCandidates(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const [running, setRunning] = useState(false)

  const approve = async (
    jobId: string,
    candidates: readonly { index: number }[],
  ): Promise<CandidateSaveOutcome> => {
    const client = createClient(MemoryService, transport)
    setRunning(true)
    try {
      const response = await client.resolveMemoryExtraction({
        jobId,
        approved: candidates.map((candidate) => candidate.index),
      })
      if (response.saved > 0) invalidateMemories(queryClient, transport, ownerId)
      return {
        saved: response.saved,
        failures: response.failures.map((failure) => ({
          index: failure.index,
          message: formatAppFailure(appFailureFromProto(failure.failure)),
        })),
      }
    } catch (cause) {
      // The whole ruling was refused (a network failure, a resolved extraction): every checked
      // row keeps the reason, and nothing was stored.
      return {
        saved: 0,
        failures: candidates.map((candidate) => ({
          index: candidate.index,
          message: memoryErrorMessage(cause),
        })),
      }
    } finally {
      setRunning(false)
    }
  }

  /** Closing the sheet discards what is left: a ruling that approves nothing, so the job's
   *  payload goes with the sheet (MEM-15). A job already resolved or never loaded has nothing
   *  to discard, which is why its refusal is not the user's to see. */
  const discard = (jobId: string) =>
    createClient(MemoryService, transport)
      .resolveMemoryExtraction({ jobId, approved: [] })
      .then(
        () => undefined,
        () => undefined,
      )

  return { approve, discard, isPending: running }
}

export type { MemoryCandidate }
