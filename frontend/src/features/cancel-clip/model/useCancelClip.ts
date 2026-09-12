import { useRef, useState } from 'react'
import { useClipLifecycleApi } from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

export function useCancelClip(ownerId: string, projectId: string, job?: GenerationJob) {
  const api = useClipLifecycleApi(ownerId, projectId)
  const lock = useRef(false)
  const acknowledged = useRef(new Set<string>())
  const [pending, setPending] = useState(false)
  const [unresolved, setUnresolved] = useState<string>()
  const [accepted, setAccepted] = useState<string>()
  const [failure, setFailure] = useState<AppFailure>()
  const uncertain = unresolved === job?.id && !!unresolved && !isTerminal(job)
  const cancelling =
    !isTerminal(job) && (!!job?.cancelRequestedAt || (accepted === job?.id && !!accepted))
  async function refresh() {
    const found = await api.refresh()
    if (
      found.latestJob?.id === job?.id &&
      (found.latestJob?.cancelRequestedAt || isTerminal(found.latestJob))
    ) {
      acknowledged.current.add(job!.id)
      setAccepted(job!.id)
    }
    setUnresolved(undefined)
  }
  async function cancel() {
    if (
      lock.current ||
      !job?.canCancel ||
      acknowledged.current.has(job.id) ||
      isTerminal(job) ||
      cancelling ||
      uncertain
    )
      return
    lock.current = true
    setPending(true)
    setFailure(undefined)
    try {
      const result = await api.cancel(job.id)
      if (result.cancelRequestedAt || isTerminal(result)) {
        acknowledged.current.add(job.id)
        setAccepted(job.id)
      }
      await refresh()
    } catch (error) {
      setFailure(appFailureFromConnect(error))
      setUnresolved(job.id)
      try {
        await refresh()
      } catch {
        /* Polling remains mounted; the retry only reads. */
      }
    } finally {
      lock.current = false
      setPending(false)
    }
  }
  async function checkAgain() {
    if (lock.current) return
    lock.current = true
    setPending(true)
    try {
      await refresh()
      setFailure(undefined)
    } catch (error) {
      setFailure(appFailureFromConnect(error))
    } finally {
      lock.current = false
      setPending(false)
    }
  }
  return { cancel, checkAgain, pending, uncertain, cancelling, failure }
}
