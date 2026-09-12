import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipService, type ProtoClipProject, type ProtoClipSourceBatch } from '@/shared/api'
import { toGenerationJob } from '@/entities/generation-job/@x/clip-project'
import { toClipEditingState } from './edit-plan'
import { toClipAccounting } from './credits'
import { toClipObservations } from './observations'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  CLIP_RATIOS,
  normalizeClipProject,
  type ClipProject,
  type ClipProjectDraft,
  type ClipRatio,
  type ClipSourceBatch,
} from '../model/types'

export const clipProjectsKey = (transport: Transport, ownerId: string) =>
  ['clip-projects', transport, ownerId] as const
export function toClipProject(value: ProtoClipProject): ClipProject {
  if (!CLIP_RATIOS.includes(value.ratio as ClipRatio)) throw new Error('Invalid clip ratio')
  return {
    id: value.id,
    title: value.title,
    videoTemplateId: value.videoTemplateId,
    ratio: value.ratio as ClipRatio,
    targetDurationMs: value.targetDurationMs,
    // An empty campaign type is a clip still being set up; generation refuses
    // one (CDS-5). An empty CTA means the template preset's (CDS-29).
    disclosure: value.disclosure as ClipProject['disclosure'],
    hideDisclosure: value.hideDisclosure,
    cta: value.cta as ClipProject['cta'],
    answers: value.answers.map((a) => ({ label: a.label, text: a.text })),
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    editPlanRevision: value.editPlanRevision,
    renderedPlanRevision: value.renderedPlanRevision,
    latestJob: value.latestJob ? toGenerationJob(value.latestJob) : undefined,
    latestAttempt:
      value.latestAttempt?.jobId && value.latestAttempt.batchId
        ? {
            jobId: value.latestAttempt.jobId,
            batchId: value.latestAttempt.batchId,
            quoteId: value.latestAttempt.quoteId,
          }
        : undefined,
    accounting: value.accounting ? toClipAccounting(value.accounting) : undefined,
    editing: value.editing ? toClipEditingState(value.editing) : undefined,
    observations: value.observations ? toClipObservations(value.observations) : undefined,
    result: value.result
      ? {
          contentType: value.result.contentType,
          bytes: Number(value.result.bytes),
          durationMs: value.result.durationMs,
          createdAt: value.result.createdAt,
          viewUrl: value.result.viewUrl || undefined,
          downloadUrl: value.result.downloadUrl || undefined,
        }
      : undefined,
  }
}
export function toClipSourceBatch(value: ProtoClipSourceBatch): ClipSourceBatch {
  if (!value.id || !['uploading', 'ready', 'consuming', 'cleanup_pending'].includes(value.state))
    throw new Error('Invalid source batch')
  return {
    id: value.id,
    projectId: value.projectId,
    state: value.state as ClipSourceBatch['state'],
    expiresAt: value.expiresAt,
    sources: value.sources.map((source) => {
      if (!source.metadata || !['pending', 'ready'].includes(source.state))
        throw new Error('Invalid source lease')
      const m = source.metadata
      return {
        id: source.id,
        state: source.state as 'pending' | 'ready',
        actualBytes: Number(source.actualBytes),
        metadata: {
          filename: m.filename,
          contentType: m.contentType,
          bytes: Number(m.bytes),
          durationMs: m.durationMs,
          width: m.width,
          height: m.height,
          fingerprint: m.fingerprint,
        },
      }
    }),
  }
}
export function useClipProjects(ownerId: string) {
  const transport = useTransport()
  return useQuery<ClipProject[]>({
    queryKey: [...clipProjectsKey(transport, ownerId), 'list'],
    enabled: !!ownerId,
    staleTime: 0,
    refetchOnMount: 'always',
    refetchInterval: (state) =>
      state.state.data?.some(
        (project) =>
          project.latestJob?.status === 'queued' || project.latestJob?.status === 'running',
      )
        ? POLL_INTERVAL_MS
        : false,
    queryFn: async ({ signal }) =>
      (await createClient(ClipService, transport).listClipProjects({}, { signal })).projects.map(
        toClipProject,
      ),
  })
}
export function useClipProject(ownerId: string, id: string | undefined) {
  const transport = useTransport()
  return useQuery<ClipProject>({
    queryKey: [...clipProjectsKey(transport, ownerId), 'detail', id],
    enabled: !!ownerId && !!id,
    staleTime: 0,
    refetchOnMount: 'always',
    refetchInterval: (state) => {
      const project = state.state.data
      const job = project?.latestJob
      if (!job) return false
      if (job.status === 'queued' || job.status === 'running') return POLL_INTERVAL_MS
      return job.kind === 'generate_clip' &&
        (!project.accounting || project.accounting.jobId !== job.id || !project.accounting.settled)
        ? POLL_INTERVAL_MS
        : false
    },
    queryFn: async ({ signal }) => {
      const response = await createClient(ClipService, transport).getClipProject({ id }, { signal })
      if (!response.project) throw new Error('Missing clip')
      return toClipProject(response.project)
    },
  })
}
export function useClipProjectMutations(ownerId: string) {
  const transport = useTransport()
  const client = createClient(ClipService, transport)
  const cache = useQueryClient()
  const invalidate = () =>
    Promise.all([
      cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) }),
      cache.invalidateQueries({ queryKey: ['clip-templates', transport, ownerId] }),
    ])
  const save = useMutation({
    mutationFn: async ({ id, draft }: { id?: string; draft: ClipProjectDraft }) => {
      const { ratio, ...fields } = normalizeClipProject(draft)
      const response = id
        ? await client.updateClipProject({ id, ...fields })
        : await client.createClipProject({ ...fields, ratio })
      if (!response.project) throw new Error('Missing saved clip')
      return toClipProject(response.project)
    },
    onSuccess: invalidate,
  })
  const remove = useMutation({
    mutationFn: (id: string) => client.deleteClipProject({ id }),
    onSuccess: invalidate,
  })
  return { save, remove }
}
