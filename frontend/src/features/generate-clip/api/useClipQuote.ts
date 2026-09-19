import { useEffect, useState } from 'react'
import {
  useClipGenerationQuote,
  type ClipEligibilityStatus,
  type ClipProject,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import type { ModelRef } from '@/entities/model-catalog'
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
  const binding = clipQuoteBinding(project, batch, observe, write, status)
  const query = useClipGenerationQuote(
    ownerId,
    { projectId: project.id, batchId: batch.id, observeModel: observe, writeModel: write },
    binding,
  )
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
