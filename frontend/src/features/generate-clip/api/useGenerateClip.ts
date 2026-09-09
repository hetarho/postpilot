import { useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { clipProjectsKey, type ClipProject, type ReadyClipBatch } from '@/entities/clip-project'
import { isTerminal, useJob } from '@/entities/generation-job'
import { myPlanQueryKey } from '@/entities/plan'
import { useSelectionSavePending, useStageSelection } from '@/entities/model-catalog'
import { ClipService, appFailureFromConnect, type AppFailure } from '@/shared/api'
import { clipModelsReady, readyClipBatch } from '../model/preconditions'

export function useGenerateClip(ownerId: string, project: ClipProject) {
  const transport = useTransport()
  const cache = useQueryClient()
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const selectionPending = useSelectionSavePending()
  const [startedId, setStartedId] = useState('')
  const [localFailure, setLocalFailure] = useState<AppFailure>()
  const starting = useRef(false)
  const consumed = useRef(new Set<string>())
  const mutation = useMutation({
    mutationFn: async (batchId: string) => {
      const response = await createClient(ClipService, transport).startClipGeneration({
        projectId: project.id,
        batchId,
        observeModel: observe.selected ?? undefined,
        writeModel: write.selected ?? undefined,
      })
      if (!response.jobId) throw new Error('Missing durable clip job')
      return response
    },
    retry: false,
  })
  const id = startedId || project.latestJob?.id || ''
  const poll = useJob(id, [clipProjectsKey(transport, ownerId), myPlanQueryKey(transport)])
  const job = poll.job ?? (project.latestJob?.id === id ? project.latestJob : undefined)
  const busy = mutation.isPending || (!!id && (!job || !isTerminal(job)))
  const modelsReady = !selectionPending && clipModelsReady(observe, write)
  const failure =
    localFailure ??
    (mutation.error
      ? appFailureFromConnect(mutation.error)
      : job?.status === 'failed'
        ? job.failure
        : undefined)

  async function start(
    batch: ReadyClipBatch | undefined,
    settingsReady: boolean,
    onOwned: () => void,
  ) {
    if (
      starting.current ||
      busy ||
      !settingsReady ||
      !modelsReady ||
      !batch ||
      consumed.current.has(batch.id)
    )
      return
    if (!readyClipBatch(batch, project.id, Date.now())) {
      setLocalFailure({ reason: 'CLIP_SOURCE_UNAVAILABLE', params: {} })
      return
    }
    setLocalFailure(undefined)
    starting.current = true
    try {
      const response = await mutation.mutateAsync(batch.id)
      consumed.current.add(batch.id)
      setStartedId(response.jobId)
      onOwned()
      void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    } catch {
      // An ambiguous response may hide an accepted job: refresh the owned snapshot,
      // never retry the paid start automatically or discard server-owned inputs.
      void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    } finally {
      starting.current = false
    }
  }
  return {
    job,
    busy,
    modelsReady,
    failure,
    starting: mutation.isPending,
    pollFailed: poll.isError,
    checkAgain: poll.refetch,
    start,
  }
}
