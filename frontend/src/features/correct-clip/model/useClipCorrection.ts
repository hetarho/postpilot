import { useEffect, useLayoutEffect, useReducer, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  acknowledgeClipCuts,
  clipDraftKey,
  clipPlanToProto,
  clipSourceSound,
  clipTimelineReducer,
  copyClipPlan,
  createClipTimeline,
  ownerCutId,
  type ClipEditPlan,
  type TimelineEdit,
  validateTimelinePlan,
  withSourceSound,
} from '@/entities/clip-plan'
import { type ClipAddCutSelection } from '@/entities/clip-observation'
import {
  clipProjectsKey,
  getClipSources,
  setClipSourceOriginalSound,
  toClipProject,
  type ClipProject,
  type ClipSourceBatch,
} from '@/entities/clip-project'
import { ClipService, appFailureFromConnect } from '@/shared/api'
import { CLIP_TIMELINE } from '@/entities/clip-design'
export interface ClipSoundSource {
  sourceId: string
  fingerprint: string
  batchId: string
  retainOriginalAudio: boolean
}

export function useClipCorrection(ownerId: string, project: ClipProject, createCutId = ownerCutId) {
  const transport = useTransport()
  const cache = useQueryClient()
  const initial = project.editing?.plan ?? { durationMs: 0, cuts: [], hook: '' }
  const [timeline, dispatch] = useReducer(clipTimelineReducer, initial, createClipTimeline)
  const [baseline, setBaseline] = useState(() => clipDraftKey(initial))
  const [revision, setRevision] = useState(project.editPlanRevision)
  const [remoteConflict, setRemoteConflict] = useState(false)
  const [soundBatch, setSoundBatch] = useState<ClipSourceBatch>()
  const soundSources = useRef(new Map<string, ClipSoundSource>())
  const saving = useRef<Promise<number> | undefined>(undefined)
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const draft = timeline.plan
  const dirty = clipDraftKey(draft) !== baseline
  const validation = project.editing
    ? validateTimelinePlan(draft, project.editing, project.observations)
    : undefined
  const current = useRef({
    draft,
    revision,
    dirty,
    valid: validation?.saveable,
    baseline,
    remoteConflict,
  })
  useLayoutEffect(() => {
    current.current = {
      draft,
      revision,
      dirty,
      valid: validation?.saveable,
      baseline,
      remoteConflict,
    }
  }, [draft, revision, dirty, validation?.saveable, baseline, remoteConflict])
  const mutation = useMutation({
    mutationFn: async ({
      plan,
      expectedRevision,
      sound,
    }: {
      plan: ClipEditPlan
      expectedRevision: number
      sound?: ClipSoundSource
    }) => {
      if (sound) {
        const { project: next, batch } = await setClipSourceOriginalSound(transport, {
          projectId: project.id,
          batchId: sound.batchId,
          sourceId: sound.sourceId,
          expectedFingerprint: sound.fingerprint,
          retainOriginalAudio: sound.retainOriginalAudio,
          expectedRevision,
        })
        if (mounted.current) setSoundBatch(batch)
        return next
      }
      const result = await createClient(ClipService, transport).saveClipEditPlan({
        projectId: project.id,
        expectedRevision,
        plan: clipPlanToProto({
          ...plan,
          sourceAudio: plan.sourceAudio?.filter((setting) =>
            plan.cuts.some(
              (cut) => cut.sourceId === setting.sourceId && cut.fingerprint === setting.fingerprint,
            ),
          ),
        }),
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
  if (
    !saving.current &&
    !mutation.isPending &&
    project.editing &&
    project.editPlanRevision > revision
  ) {
    if (!dirty) adopt(project)
    else if (!remoteConflict) setRemoteConflict(true)
  }
  const change = (edit: TimelineEdit, group?: string) => {
    if ((!project.editing && edit.type !== 'sourceSound') || active) return
    if (mutation.error && appFailureFromConnect(mutation.error).reason !== 'CLIP_PLAN_CONFLICT')
      mutation.reset()
    dispatch({ type: 'edit', edit, group, at: Date.now() })
  }
  function saveOne(): Promise<number> {
    if (saving.current) return saving.current
    const submitted = current.current
    if (submitted.remoteConflict) return Promise.reject(new Error('Conflicting clip draft'))
    if (!submitted.dirty) return Promise.resolve(submitted.revision)
    const accepted = JSON.parse(submitted.baseline) as ClipEditPlan
    const source = [...soundSources.current.values()].find(
      (source) =>
        clipSourceSound(submitted.draft, source, source.retainOriginalAudio) !==
        clipSourceSound(accepted, source, source.retainOriginalAudio),
    )
    if ((!submitted.valid && !source) || active)
      return Promise.reject(new Error('Invalid clip draft'))
    const sound = source
      ? {
          ...source,
          retainOriginalAudio: clipSourceSound(submitted.draft, source, source.retainOriginalAudio),
        }
      : undefined
    const snapshot = copyClipPlan(submitted.draft)
    const key = clipDraftKey(snapshot)
    const operation = (async () => {
      try {
        const result = await mutation.mutateAsync({
          plan: snapshot,
          expectedRevision: submitted.revision,
          sound,
        })
        if (!mounted.current) throw new Error('Correction was unmounted')
        let acceptedPlan = result.editing?.plan ?? accepted
        // Unused current sources have durable settings too, but are absent from
        // the server's cut-only render snapshot. Keep them in local history only.
        for (const setting of accepted.sourceAudio ?? []) {
          if (
            !acceptedPlan.cuts.some(
              (c) => c.sourceId === setting.sourceId && c.fingerprint === setting.fingerprint,
            )
          )
            acceptedPlan = withSourceSound(acceptedPlan, setting)
        }
        if (sound) acceptedPlan = withSourceSound(acceptedPlan, sound)
        let queued = current.current.draft
        if (sound) {
          // The audio RPC saves only permission. Preserve cuts/text typed before or
          // during IO, and any newer permission intent (including undo during IO).
          const intended = [...soundSources.current.values()].map((source) => ({
            ...source,
            retainOriginalAudio: clipSourceSound(queued, source, source.retainOriginalAudio),
          }))
          queued = { ...queued, sourceAudio: acceptedPlan.sourceAudio }
          for (const setting of intended) queued = withSourceSound(queued, setting)
        } else {
          queued =
            clipDraftKey(queued) === key ? acceptedPlan : acknowledgeClipCuts(queued, acceptedPlan)
        }
        dispatch({ type: 'adopt', plan: queued })
        const acceptedKey = clipDraftKey(acceptedPlan)
        current.current = {
          ...current.current,
          draft: queued,
          baseline: acceptedKey,
          revision: result.editPlanRevision,
          dirty: clipDraftKey(queued) !== acceptedKey,
        }
        setBaseline(acceptedKey)
        setRevision(result.editPlanRevision)
        publish(result)
        return result.editPlanRevision
      } finally {
        saving.current = undefined
      }
    })()
    saving.current = operation
    return operation
  }
  // UI autosave keeps the draft on failure; a committing action must receive the failure.
  const save = () => flush().catch(() => undefined)
  async function flush(): Promise<number> {
    if (saving.current) await saving.current
    while (current.current.dirty) await saveOne()
    if (current.current.remoteConflict) throw new Error('Conflicting clip draft')
    return current.current.revision
  }
  const saveRef = useRef(save)
  useLayoutEffect(() => {
    saveRef.current = save
  })
  const active =
    !!project.finalized ||
    project.latestJob?.status === 'queued' ||
    project.latestJob?.status === 'running'
  useEffect(() => {
    if (
      !dirty ||
      (!validation?.saveable &&
        ![...soundSources.current.values()].some(
          (source) =>
            clipSourceSound(draft, source, source.retainOriginalAudio) !==
            clipSourceSound(
              JSON.parse(baseline) as ClipEditPlan,
              source,
              source.retainOriginalAudio,
            ),
        )) ||
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
    baseline,
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
      if (!result.project) throw new Error('Missing current correction')
      const batches = soundSources.current.size ? await getClipSources(transport, project.id) : []
      const next = toClipProject(result.project)
      let accepted = next.editing?.plan ?? { durationMs: 0, cuts: [], hook: '' }
      for (const batch of batches.filter((b) => b.current))
        for (const source of batch.sources)
          if (soundSources.current.has(source.metadata.fingerprint))
            accepted = withSourceSound(accepted, {
              sourceId: source.id,
              fingerprint: source.metadata.fingerprint,
              retainOriginalAudio: source.retainOriginalAudio,
            })
      return { next, keepDraft, accepted }
    },
    retry: false,
    onSuccess: ({ next, keepDraft, accepted }) => {
      if (keepDraft) {
        setRevision(next.editPlanRevision)
        setBaseline(clipDraftKey(accepted))
        setRemoteConflict(false)
      } else {
        dispatch({ type: 'adopt', plan: accepted, clearHistory: true })
        setRevision(next.editPlanRevision)
        setBaseline(clipDraftKey(accepted))
        setRemoteConflict(false)
      }
      mutation.reset()
      publish(next)
    },
  })
  const audioPlan =
    mutation.error || remoteConflict
      ? { ...draft, sourceAudio: (JSON.parse(baseline) as ClipEditPlan).sourceAudio }
      : draft
  return {
    soundBatch,
    previewPlan: {
      ...audioPlan,
      sourceAudio: audioPlan.sourceAudio?.filter((setting) =>
        audioPlan.cuts.some(
          (cut) => cut.sourceId === setting.sourceId && cut.fingerprint === setting.fingerprint,
        ),
      ),
    },
    hasUnsavedSound: [...soundSources.current.values()].some(
      (source) =>
        clipSourceSound(draft, source, source.retainOriginalAudio) !==
        clipSourceSound(JSON.parse(baseline) as ClipEditPlan, source, source.retainOriginalAudio),
    ),
    soundValue: (source: ClipSoundSource) =>
      clipSourceSound(
        audioPlan,
        source,
        soundSources.current.get(source.fingerprint)?.retainOriginalAudio ??
          source.retainOriginalAudio,
      ),
    setSourceSound: (source: ClipSoundSource, enabled: boolean) => {
      if (active) return
      const prior = soundSources.current.get(source.fingerprint)
      soundSources.current.set(source.fingerprint, {
        ...source,
        retainOriginalAudio: prior?.retainOriginalAudio ?? source.retainOriginalAudio,
      })
      if (
        clipSourceSound(
          current.current.draft,
          source,
          prior?.retainOriginalAudio ?? source.retainOriginalAudio,
        ) === enabled
      )
        return
      change({
        type: 'sourceSound',
        setting: {
          sourceId: source.sourceId,
          fingerprint: source.fingerprint,
          retainOriginalAudio: enabled,
        },
      })
    },
    addCut: async (selection: ClipAddCutSelection, retainedSound?: boolean) => {
      await flush()
      const plan = current.current.draft
      const selected = timeline.selection
      const originId =
        selected?.kind === 'cut'
          ? selected.id
          : plan.elements?.find((text) => text.instanceId === selected?.id)?.cutId
      const origin = plan.cuts.find((c) => c.id === originId) ?? plan.cuts[0]
      if (
        !origin ||
        !selection.segment.focal ||
        selection.segment.usability === 'unusable' ||
        selection.startMs < selection.segment.startMs ||
        selection.endMs > selection.segment.endMs
      )
        return
      change({
        type: 'addCut',
        id: createCutId(),
        originCutId: origin.id,
        sourceId: selection.source.id,
        fingerprint: selection.source.fingerprint,
        startMs: selection.startMs,
        endMs: selection.endMs,
        focal: selection.segment.focal,
        retainedSound,
      })
    },
    splitCut: async (id: string, sourceMs: number) => {
      await flush()
      change({ type: 'splitCut', id, newId: createCutId(), sourceMs })
    },
    draft,
    timeline,
    dispatch,
    revision,
    dirty,
    validation,
    change,
    save,
    flush,
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
