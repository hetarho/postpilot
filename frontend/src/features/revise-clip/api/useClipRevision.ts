import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  useClipPlanCalls,
  useClipRevisionQuote,
  type ClipRevisionTarget,
} from '@/entities/clip-plan'
import {
  projectDraft,
  useRefreshClipProjects,
  type ClipProject,
  type ClipQuote,
} from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import type { ModelRef } from '@/entities/model-catalog'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { CLIP_REVISION } from '@/entities/clip-design'
import { POLL_INTERVAL_MS } from '@/shared/config'
/** What a revision quote is bound to: the plan the owner is looking at, the
 *  settings the writer reads beside it, and the exact words they wrote. The
 *  server digests the same set (CLIP-131), so anything that moves here has to
 *  be priced again before it can be approved. */
function revisionBinding(
  project: ClipProject,
  request: string,
  target: ClipRevisionTarget,
  observe: ModelRef | null,
  write: ModelRef | null,
) {
  return JSON.stringify([
    project.id,
    project.editPlanRevision,
    projectDraft(project),
    request,
    target,
    observe,
    write,
  ])
}

/** One owner-written revision of the saved plan: priced, approved and started
 *  from ②'s own panel (CLIP-131, CLIP-40).
 *
 *  It polls nothing. The job it starts is the page's — the same `useJob` ①'s
 *  generation already runs (CLIP-36) — and is passed back in so a step change
 *  cannot remount a second poll over the same work. */
export function useClipRevision({
  ownerId,
  project,
  request,
  target,
  observe,
  write,
  job,
}: {
  ownerId: string
  project: ClipProject
  request: string
  target: ClipRevisionTarget
  observe: ModelRef | null
  write: ModelRef | null
  job?: GenerationJob
}) {
  const calls = useClipPlanCalls()
  const refresh = useRefreshClipProjects(ownerId)
  // A quote binds the words it was taken against, so the text is part of its
  // key; the settle is what keeps that from being one request per character.
  const [settled, setSettled] = useState(request)
  useEffect(() => {
    if (settled === request) return
    const timer = window.setTimeout(() => setSettled(request), CLIP_REVISION.quoteDebounceMs)
    return () => window.clearTimeout(timer)
  }, [request, settled])
  const [failure, setFailure] = useState<AppFailure>()
  const mine = job?.kind === 'revise_clip' ? job : undefined
  const text = settled.trim()
  const binding = revisionBinding(project, text, target, observe, write)
  const mutation = useMutation({
    mutationFn: async (quote: ClipQuote) => {
      return calls.revise({
        projectId: project.id,
        request: text,
        target,
        observeModel: observe!,
        writeModel: write!,
        quote,
      })
    },
    // Never replay a paid request, and never pause one offline to resume later.
    retry: false,
    networkMode: 'always',
    gcTime: 0,
  })
  // An accepted request runs until the job says otherwise. The last clause is the
  // gap between the start returning and the page's projection carrying the job it
  // minted: the panel must not offer the field again in it.
  const running =
    mutation.isPending || !!(mine && !isTerminal(mine)) || (mutation.isSuccess && !job)
  const query = useClipRevisionQuote(
    ownerId,
    { projectId: project.id, request: text, target, observeModel: observe, writeModel: write },
    binding,
    !!text &&
      !!observe &&
      !!write &&
      !project.finalized &&
      project.editPlanRevision > 0 &&
      !running,
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
    running,
    quoting: query.isFetching || request !== settled,
    expired,
    quote:
      query.data?.binding === binding && !expired && !query.isFetching && !query.isError
        ? query.data
        : undefined,
    error: query.error ? appFailureFromConnect(query.error) : undefined,
    failure: failure ?? (mine?.status === 'failed' ? mine.failure : undefined),
    cancelled: mine?.status === 'cancelled',
    job: mine,
    refresh: () => void query.refetch(),
    /** Sends the approved request. `flush` writes the owner's unsaved edits
     *  first and answers with the plan revision they landed on: the writer is
     *  fed the saved plan (T201), so an edit left in the draft would simply be
     *  dropped from what it rewrites. */
    start: async (quote: ClipQuote, flush: () => Promise<number>) => {
      if (running || !text) return
      setFailure(undefined)
      let revision: number
      try {
        revision = await flush()
      } catch {
        // The failed save states itself on ②'s own line; the request stays put.
        return
      }
      // A flush that saved something moved the plan this ceiling was priced
      // against. Re-quote rather than rewrite a plan nobody approved.
      if (revision !== project.editPlanRevision) {
        setFailure({ reason: 'CLIP_QUOTE_CHANGED', params: {} })
        void refresh.all()
        return
      }
      try {
        await mutation.mutateAsync(quote)
      } catch (error) {
        setFailure(appFailureFromConnect(error))
      }
      void refresh.all()
    },
  }
}
