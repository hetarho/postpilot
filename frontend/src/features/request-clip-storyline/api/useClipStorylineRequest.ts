import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { CLIP_REVISION } from '@/entities/clip-design'
import {
  useClipProjectCalls,
  useClipStorylineRevisionQuote,
  useRefreshClipProjects,
  type ClipProject,
  type ClipQuote,
} from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import type { ModelRef } from '@/entities/model-catalog'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'

/** One storyline request (CLIP-181): priced against the storyline the owner is looking at and the
 *  words they wrote, approved and started from the space's own field. It polls nothing: the job it
 *  starts is the page's, passed back in. */
export function useClipStorylineRequest({
  ownerId,
  project,
  request,
  observe,
  write,
  job,
}: {
  ownerId: string
  project: ClipProject
  request: string
  observe: ModelRef | null
  write: ModelRef | null
  job?: GenerationJob
}) {
  const calls = useClipProjectCalls()
  const refresh = useRefreshClipProjects(ownerId)
  // A quote binds the words it was taken against; the settle keeps that from being one request
  // per keystroke.
  const [settled, setSettled] = useState(request)
  useEffect(() => {
    if (settled === request) return
    const timer = window.setTimeout(() => setSettled(request), CLIP_REVISION.quoteDebounceMs)
    return () => window.clearTimeout(timer)
  }, [request, settled])
  const [failure, setFailure] = useState<AppFailure>()
  const mine = job?.kind === 'revise_storyline_clip' ? job : undefined
  const text = settled.trim()
  const binding = JSON.stringify([project.id, project.storyline, text, observe, write])
  const mutation = useMutation({
    mutationFn: (quote: ClipQuote) =>
      calls.startStorylineRevision({
        projectId: project.id,
        request: text,
        observeModel: observe!,
        writeModel: write!,
        quoteId: quote.quoteId,
        approvedMaxCredits: quote.maxCredits,
        cancellationPolicyVersion: quote.cancellationPolicy?.version,
      }),
    // Never replay a paid request, and never pause one offline to resume later.
    retry: false,
    networkMode: 'always',
    gcTime: 0,
  })
  const running =
    mutation.isPending || !!(mine && !isTerminal(mine)) || (mutation.isSuccess && !job)
  const query = useClipStorylineRevisionQuote(
    ownerId,
    { projectId: project.id, request: text, observeModel: observe, writeModel: write },
    binding,
    !!text && !!observe && !!write && !project.finalized && !!project.storyline && !running,
  )
  const [now, setNow] = useState(Date.now)
  const expires = Date.parse(query.data?.expiresAt ?? '')
  useEffect(() => {
    if (!Number.isFinite(expires) || expires <= now) return
    const timer = window.setTimeout(
      () => setNow(Date.now()),
      Math.max(0, Math.min(POLL_INTERVAL_MS, expires - Date.now())),
    )
    return () => window.clearTimeout(timer)
  }, [expires, now])
  const expired = !!query.data && expires <= now
  return {
    running,
    quoting: query.isFetching || request !== settled,
    expired,
    quote:
      query.data?.binding === binding && !expired && !query.isFetching && !query.isError
        ? query.data
        : undefined,
    error: query.error ? appFailureFromConnect(query.error) : undefined,
    failure: failure ?? (mine?.status === 'failed' ? mine.failure : undefined),
    job: mine,
    refresh: () => void query.refetch(),
    start: async (quote: ClipQuote) => {
      if (running || !text) return
      setFailure(undefined)
      try {
        await mutation.mutateAsync(quote)
      } catch (error) {
        setFailure(appFailureFromConnect(error))
      }
      void refresh.all()
    },
  }
}
