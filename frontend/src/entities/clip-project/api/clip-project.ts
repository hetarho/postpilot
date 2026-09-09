import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipService, type ProtoClipProject, type ProtoClipSourceBatch } from '@/shared/api'
import { toGenerationJob } from '@/entities/generation-job/@x/clip-project'
import { toClipEditingState } from './edit-plan'
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
    answers: value.answers.map((a) => ({ label: a.label, text: a.text })),
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    editPlanRevision: value.editPlanRevision,
    renderedPlanRevision: value.renderedPlanRevision,
    latestJob: value.latestJob ? toGenerationJob(value.latestJob) : undefined,
    editing: value.editing ? toClipEditingState(value.editing) : undefined,
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
  return useQuery({
    queryKey: [...clipProjectsKey(transport, ownerId), 'list'],
    enabled: !!ownerId,
    staleTime: 0,
    refetchOnMount: 'always',
    queryFn: async () =>
      (await createClient(ClipService, transport).listClipProjects({})).projects.map(toClipProject),
  })
}
export function useClipProject(ownerId: string, id: string | undefined) {
  const transport = useTransport()
  return useQuery({
    queryKey: [...clipProjectsKey(transport, ownerId), 'detail', id],
    enabled: !!ownerId && !!id,
    staleTime: 0,
    refetchOnMount: 'always',
    queryFn: async () => {
      const response = await createClient(ClipService, transport).getClipProject({ id })
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
