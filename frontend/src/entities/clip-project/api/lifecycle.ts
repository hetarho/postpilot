import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ClipGenerationService } from '@/shared/api'
import { toGenerationJob } from '@/entities/generation-job/@x/clip-project'
import { clipProjectsKey, toClipProject } from './clip-project'
import type { ClipProject } from '../model/types'

/** Explicit reads reconcile potentially committed writes before another action is allowed. */
export function useClipLifecycleApi(ownerId: string, projectId: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  const client = createClient(ClipGenerationService, transport)
  const key = [...clipProjectsKey(transport, ownerId), 'detail', projectId]
  function settle(project: ClipProject) {
    cache.setQueryData(key, project)
    void cache.invalidateQueries({ queryKey: [...clipProjectsKey(transport, ownerId), 'list'] })
    return project
  }
  async function refresh() {
    await cache.cancelQueries({ queryKey: key })
    const response = await client.getClipProject({ id: projectId })
    if (!response.project) throw new Error('Missing owned clip')
    return settle(toClipProject(response.project))
  }
  return {
    refresh,
    /** 확정하기 (CLIP-75): records the saved plan revision and answers the project as the detail
     *  reads it. A plan saved since is the request's CLIP_PLAN_CONFLICT. */
    confirm: async (expectedPlanRevision: number) => {
      const response = await client.confirmClipProject({ projectId, expectedPlanRevision })
      if (!response.project) throw new Error('Missing confirmed clip')
      return settle(toClipProject(response.project))
    },
    /** Finalization (CLIP-76, CLIP-232): with a posted URL the clip becomes 게시됨, without one
     *  완료. The expected revision and result fence the plan and render the dialog showed. */
    finalize: async (input: {
      expectedRevision: number
      expectedResultId: string
      publishedUrl?: string
    }) => {
      const response = await client.finalizeClipProject({
        projectId,
        expectedRevision: input.expectedRevision,
        expectedResultId: input.expectedResultId,
        publishedUrl: input.publishedUrl ?? '',
      })
      if (!response.project) throw new Error('Missing finalized clip')
      const project = toClipProject(response.project)
      if (!project.finalized) throw new Error('Clip finalization was not confirmed')
      return settle(project)
    },
    /** Saves or replaces a finalized clip's posted URL, which changes nothing else (CLIP-232). */
    savePublishedUrl: async (url: string) => {
      const response = await client.saveClipPublishedUrl({ projectId, url })
      if (!response.project) throw new Error('Missing clip')
      return settle(toClipProject(response.project))
    },
    cancel: async (jobId: string, signal?: AbortSignal) => {
      const response = await client.cancelClipJob({ projectId, jobId }, { signal })
      if (!response.job) throw new Error('Missing cancelled clip job')
      return toGenerationJob(response.job)
    },
  }
}
