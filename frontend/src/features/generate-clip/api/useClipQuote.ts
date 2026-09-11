import { useEffect, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import {
  toClipQuote,
  type ClipEligibilityStatus,
  type ClipProject,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import type { ModelRef } from '@/entities/model-catalog'
import { ClipService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { clipQuoteBinding } from '../model/preconditions'

// Mounted only while saved inputs are eligible. Unmount discards the quote, so
// editing then reverting to identical settings still requires a newly read quote.
export function useClipQuote(
  ownerId: string,
  project: ClipProject,
  batch: ReadyClipBatch,
  observe: ModelRef,
  write: ModelRef,
  status: ClipEligibilityStatus | undefined,
) {
  const transport = useTransport()
  const binding = clipQuoteBinding(project, batch, observe, write, status)
  const query = useQuery({
    queryKey: ['clip-quote', transport, ownerId, binding],
    gcTime: 0,
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    queryFn: async ({ signal }) =>
      toClipQuote(
        await createClient(ClipService, transport).quoteClipGeneration(
          { projectId: project.id, batchId: batch.id, observeModel: observe, writeModel: write },
          { signal },
        ),
        binding,
      ),
  })
  const [now, setNow] = useState(Date.now)
  const expires = Math.min(Date.parse(query.data?.expiresAt ?? ''), Date.parse(batch.expiresAt))
  useEffect(() => {
    if (!Number.isFinite(expires) || expires <= now) return
    const timer = window.setTimeout(
      () => setNow(Date.now()),
      Math.max(0, Math.min(POLL_INTERVAL_MS, expires - Date.now())),
    )
    return () => window.clearTimeout(timer)
  }, [expires, now])
  useEffect(() => {
    const resume = () => setNow(Date.now())
    window.addEventListener('focus', resume)
    document.addEventListener('visibilitychange', resume)
    return () => {
      window.removeEventListener('focus', resume)
      document.removeEventListener('visibilitychange', resume)
    }
  }, [])
  const expired = !!query.data && expires <= now
  return {
    ...query,
    expired,
    quote:
      query.data?.binding === binding && !expired && !query.isFetching && !query.isError
        ? query.data
        : undefined,
  }
}
