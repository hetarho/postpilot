import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ClipGenerationService } from '@/shared/api'
import { toGenerationJob } from '@/entities/generation-job/@x/clip-project'
import { clipProjectsKey, toClipProject } from './clip-project'

/** Explicit reads reconcile potentially committed writes before another action is allowed. */
export function useClipLifecycleApi(ownerId: string, projectId: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  const client = createClient(ClipGenerationService, transport)
  const key = [...clipProjectsKey(transport, ownerId), 'detail', projectId]
  async function refresh() {
    await cache.cancelQueries({ queryKey: key })
    const response = await client.getClipProject({ id: projectId })
    if (!response.project) throw new Error('Missing owned clip')
    const project = toClipProject(response.project)
    cache.setQueryData(key, project)
    void cache.invalidateQueries({ queryKey: [...clipProjectsKey(transport, ownerId), 'list'] })
    return project
  }
  return {
    refresh,
    /** 확정하기 (CLIP-75): records the saved plan revision and answers the project as the detail
     *  reads it. A plan saved since is the request's CLIP_PLAN_CONFLICT. */
    confirm: async (expectedPlanRevision: number) => {
      const response = await client.confirmClipProject({ projectId, expectedPlanRevision })
      if (!response.project) throw new Error('Missing confirmed clip')
      const project = toClipProject(response.project)
      cache.setQueryData(key, project)
      void cache.invalidateQueries({ queryKey: [...clipProjectsKey(transport, ownerId), 'list'] })
      return project
    },
    cancel: async (jobId: string, signal?: AbortSignal) => {
      const response = await client.cancelClipJob({ projectId, jobId }, { signal })
      if (!response.job) throw new Error('Missing cancelled clip job')
      return toGenerationJob(response.job)
    },
  }
}
