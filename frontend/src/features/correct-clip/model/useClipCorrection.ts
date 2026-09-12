import { useEffect, useLayoutEffect, useReducer, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  clipPlanToProto,
  clipProjectsKey,
  clipDraftKey,
  copyClipPlan,
  createClipTimeline,
  clipTimelineReducer,
  toClipProject,
  validateTimelinePlan,
  type ClipProject,
  type ClipEditPlan,
  type TimelineEdit,
} from '@/entities/clip-project'
import { ClipService, appFailureFromConnect } from '@/shared/api'
import { CLIP_TIMELINE } from '@/shared/config'

export function useClipCorrection(ownerId: string, project: ClipProject) {
  const transport = useTransport()
  const cache = useQueryClient()
  const initial = project.editing?.plan ?? { durationMs: 0, cuts: [], hook: '' }
  const [timeline, dispatch] = useReducer(clipTimelineReducer, initial, createClipTimeline)
  const [baseline, setBaseline] = useState(() => clipDraftKey(initial))
  const [revision, setRevision] = useState(project.editPlanRevision)
  const [remoteConflict, setRemoteConflict] = useState(false)
  const saving = useRef<Promise<void> | undefined>(undefined)
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const draft = timeline.plan
  const dirty = clipDraftKey(draft) !== baseline
  const validation = project.editing ? validateTimelinePlan(draft, project.editing) : undefined
  const current = useRef({ draft, revision, dirty, valid: validation?.saveable, baseline })
  useLayoutEffect(() => {
    current.current = { draft, revision, dirty, valid: validation?.saveable, baseline }
  }, [draft, revision, dirty, validation?.saveable, baseline])
  const mutation = useMutation({
    mutationFn: async ({
      plan,
      expectedRevision,
    }: {
      plan: ClipEditPlan
      expectedRevision: number
    }) => {
      const result = await createClient(ClipService, transport).saveClipEditPlan({
        projectId: project.id,
        expectedRevision,
        plan: clipPlanToProto(plan),
      })
      if (!result.project?.editing) throw new Error('Missing saved correction')
      return toClipProject(result.project)
    },
    retry: false,
  })
  function publish(next: ClipProject) {
    cache.setQueryData([...clipProjectsKey(transport, ownerId), 'detail', project.id], next)
    void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
  }
  function adopt(next: ClipProject, clearHistory = false) {
    if (!next.editing) return
    dispatch({ type: 'adopt', plan: next.editing.plan, clearHistory })
    setBaseline(clipDraftKey(next.editing.plan))
    setRevision(next.editPlanRevision)
    setRemoteConflict(false)
  }
  // Never let an older query response replace a newer accepted save or draft.
  if (!mutation.isPending && project.editing && project.editPlanRevision > revision) {
    if (!dirty) adopt(project)
    else if (!remoteConflict) setRemoteConflict(true)
  }
  const change = (edit: TimelineEdit, group?: string) => {
    if (!project.editing) return
    if (mutation.error && appFailureFromConnect(mutation.error).reason !== 'CLIP_PLAN_CONFLICT')
      mutation.reset()
    dispatch({ type: 'edit', edit, group, at: Date.now() })
  }
  function save() {
    if (saving.current) return saving.current
    const submitted = current.current
    if (!submitted.dirty || !submitted.valid || remoteConflict) return Promise.resolve()
    const snapshot = copyClipPlan(submitted.draft)
    const key = clipDraftKey(snapshot)
    const operation = (async () => {
      try {
        const result = await mutation.mutateAsync({
          plan: snapshot,
          expectedRevision: submitted.revision,
        })
        if (!mounted.current || !result.editing) return
        // Typing remains enabled during IO. Accept only the submitted snapshot;
        // newer local edits remain queued against the new optimistic revision.
        if (clipDraftKey(current.current.draft) === key) {
          dispatch({ type: 'adopt', plan: result.editing.plan })
        }
        setBaseline(clipDraftKey(result.editing.plan))
        setRevision(result.editPlanRevision)
        publish(result)
      } catch {
        /* Preserve every local edit and wait for an explicit retry. */
      } finally {
        saving.current = undefined
      }
    })()
    saving.current = operation
    return operation
  }
  const saveRef = useRef(save)
  useLayoutEffect(() => {
    saveRef.current = save
  })
  const active = project.latestJob?.status === 'queued' || project.latestJob?.status === 'running'
  useEffect(() => {
    if (
      !dirty ||
      !validation?.saveable ||
      mutation.isPending ||
      mutation.error ||
      remoteConflict ||
      active
    )
      return
    const timer = setTimeout(() => {
      void saveRef.current()
    }, CLIP_TIMELINE.autosaveMs)
    return () => clearTimeout(timer)
  }, [
    draft,
    dirty,
    validation?.saveable,
    mutation.isPending,
    mutation.error,
    remoteConflict,
    active,
  ])
  const reload = useMutation({
    mutationFn: async (keepDraft: boolean) => {
      const result = await createClient(ClipService, transport).getClipProject({ id: project.id })
      if (!result.project?.editing) throw new Error('Missing current correction')
      return { next: toClipProject(result.project), keepDraft }
    },
    retry: false,
    onSuccess: ({ next, keepDraft }) => {
      if (keepDraft) {
        setRevision(next.editPlanRevision)
        setBaseline(clipDraftKey(next.editing!.plan))
        setRemoteConflict(false)
      } else adopt(next, true)
      mutation.reset()
      publish(next)
    },
  })
  return {
    draft,
    timeline,
    dispatch,
    revision,
    dirty,
    validation,
    change,
    save,
    reset: () => {
      adopt(project, true)
      mutation.reset()
    },
    pending: mutation.isPending || reload.isPending,
    saving: mutation.isPending,
    failure: remoteConflict
      ? { reason: 'CLIP_PLAN_CONFLICT' as const, params: {} }
      : mutation.error
        ? appFailureFromConnect(mutation.error)
        : reload.error
          ? appFailureFromConnect(reload.error)
          : undefined,
    reload: () => reload.mutate(false),
    reapply: () => reload.mutate(true),
  }
}
