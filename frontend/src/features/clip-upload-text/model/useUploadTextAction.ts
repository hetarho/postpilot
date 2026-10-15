import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import {
  useClipUploadTextCalls,
  type ClipChannel,
  type ClipProject,
  type ClipQuote,
} from '@/entities/clip-project'
import { useJob, isTerminal, type ModelRef } from '@/entities/generation-job'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'

export function useUploadTextAction({
  ownerId,
  project,
  channel,
  writeModel,
  revision,
  beforeWrite,
}: {
  ownerId: string
  project: ClipProject
  channel: ClipChannel
  writeModel: ModelRef | null
  revision: number
  beforeWrite?: () => Promise<unknown>
}) {
  const calls = useClipUploadTextCalls(ownerId)
  const binding = JSON.stringify([
    ownerId,
    project.id,
    channel,
    revision,
    project.updatedAt,
    project.editPlanRevision,
    project.campaign,
    writeModel,
  ])
  const epochRef = useRef(0)
  const held = useRef(false)
  const abort = useRef<AbortController | undefined>(undefined)
  const [identity, setIdentity] = useState(binding)
  const [quote, setQuote] = useState<ClipQuote>()
  const [confirmed, setConfirmed] = useState(false)
  const [phase, setPhase] = useState<'idle' | 'quoting' | 'starting' | 'saving'>('idle')
  const [failure, setFailure] = useState<AppFailure>()
  const [jobId, setJobId] = useState('')
  const [expired, setExpired] = useState(false)
  if (identity !== binding) {
    setIdentity(binding)
    setQuote(undefined)
    setConfirmed(false)
    setPhase('idle')
    setFailure(undefined)
    setExpired(false)
  }
  useLayoutEffect(() => {
    epochRef.current += 1
    held.current = false
    return () => {
      epochRef.current += 1
      held.current = false
      abort.current?.abort()
    }
  }, [binding, calls])
  useEffect(() => {
    if (!quote) return
    let timer: number
    const check = () => {
      const remaining = Date.parse(quote.expiresAt) - Date.now()
      if (remaining <= 0) setExpired(true)
      else timer = window.setTimeout(check, Math.min(POLL_INTERVAL_MS, remaining))
    }
    check()
    return () => window.clearTimeout(timer)
  }, [quote])
  const latest = project.latestJob?.kind === 'clip_upload_text' ? project.latestJob : undefined
  const tracked = useJob(
    latest?.id && latest.id !== jobId ? latest.id : jobId || latest?.id || '',
    calls.keys,
  )
  const job = tracked.job ?? latest
  const running = !!job && !isTerminal(job)
  const otherBusy =
    !!project.latestJob &&
    !isTerminal(project.latestJob) &&
    project.latestJob.kind !== 'clip_upload_text'
  const busy = phase !== 'idle' || running || otherBusy
  const guard = () => {
    const own = epochRef.current
    return () => epochRef.current === own
  }
  const prepare = async (confirmEdited: boolean) => {
    if (held.current || busy || !writeModel) return
    held.current = true
    const current = guard()
    setFailure(undefined)
    setQuote(undefined)
    setExpired(false)
    setPhase('quoting')
    const controller = new AbortController()
    abort.current = controller
    try {
      await beforeWrite?.()
      if (!current()) return
      const next = await calls.quote(
        { projectId: project.id, channel, writeModel, confirmEdited },
        binding,
        controller.signal,
      )
      if (!current()) return
      setConfirmed(confirmEdited)
      setQuote(next)
    } catch (error) {
      if (current()) setFailure(appFailureFromConnect(error))
    } finally {
      if (current()) {
        held.current = false
        setPhase('idle')
      }
    }
  }
  const start = async (approved: ClipQuote) => {
    if (
      held.current ||
      busy ||
      !writeModel ||
      expired ||
      approved !== quote ||
      approved.binding !== binding ||
      Date.parse(approved.expiresAt) <= Date.now()
    )
      return
    held.current = true
    const current = guard()
    setFailure(undefined)
    setPhase('starting')
    try {
      await beforeWrite?.()
      if (!current()) return
      const id = await calls.start(
        { projectId: project.id, channel, writeModel, confirmEdited: confirmed },
        approved,
      )
      if (current()) {
        setJobId(id)
        setQuote(undefined)
      }
    } catch (error) {
      if (current()) setFailure(appFailureFromConnect(error))
    } finally {
      if (current()) {
        held.current = false
        setPhase('idle')
      }
    }
  }
  const save = async (body: string, tags: string[], expectedRevision: number) => {
    if (held.current || busy) return false
    held.current = true
    const current = guard()
    setPhase('saving')
    setFailure(undefined)
    try {
      await calls.save({ projectId: project.id, channel, body, tags, expectedRevision })
      return true
    } catch (error) {
      if (current()) setFailure(appFailureFromConnect(error))
      return false
    } finally {
      if (current()) {
        held.current = false
        setPhase('idle')
      }
    }
  }
  return {
    quote: quote?.binding === binding ? quote : undefined,
    phase,
    expired,
    busy,
    running,
    job,
    failure: failure ?? (job?.status === 'failed' ? job.failure : undefined),
    prepare,
    start,
    save,
    dismiss: () => setQuote(undefined),
    refresh: () => void prepare(confirmed),
    cancel: async () => {
      if (!job || isTerminal(job) || held.current) return
      held.current = true
      const current = guard()
      try {
        await calls.cancel(project.id, job.id)
      } catch (error) {
        if (current()) setFailure(appFailureFromConnect(error))
      } finally {
        if (current()) held.current = false
      }
    },
  }
}
