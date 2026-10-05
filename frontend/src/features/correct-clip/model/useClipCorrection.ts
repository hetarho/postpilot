import { splitAtOutput, type SourceBounds } from './timeline-gesture'
import { useEffect, useLayoutEffect, useMemo, useReducer, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  acknowledgeClipCuts,
  clipDraftKey,
  clipSourceSound,
  clipTimelineReducer,
  copyClipPlan,
  createClipTimeline,
  ownerCutId,
  timelineCuts,
  sourceToOutputMs,
  rebaseClipRegions,
  type ClipEditPlan,
  type TimelineEdit,
  validateTimelinePlan,
  withSourceSound,
} from '@/entities/clip-plan'
import { type ClipAddCutSelection } from '@/entities/clip-observation'
import {
  serialClipWrite,
  useClipProjectCalls,
  useClipProjectsKey,
  useClipSourceCalls,
  type ClipProject,
  type ClipSourceBatch,
} from '@/entities/clip-project'
import { useClipPlanCalls } from '@/entities/clip-plan'
import { appFailureFromConnect } from '@/shared/api'
import { CLIP_TIMELINE } from '@/entities/clip-design'
export interface ClipSoundSource {
  sourceId: string
  fingerprint: string
  batchId: string
  retainOriginalAudio: boolean
}

export function useClipCorrection(ownerId: string, project: ClipProject, createCutId = ownerCutId) {
  const calls = useClipPlanCalls()
  const projects = useClipProjectCalls()
  const projectsKey = useClipProjectsKey(ownerId)
  const sources = useClipSourceCalls()
  const cache = useQueryClient()
  const initial = project.editing?.plan ?? { durationMs: 0, cuts: [] }
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
  const [transientPlan, setTransientPlan] = useState<ClipEditPlan>()
  const draft = timeline.plan
  const dirty = clipDraftKey(draft) !== baseline
  const validation = project.editing
    ? validateTimelinePlan(
        draft,
        project.editing,
        project.observations,
        project.allowedCaptionStyles,
      )
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
        const { project: next, batch } = await sources.sound({
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
      return calls.save({
        projectId: project.id,
        expectedRevision,
        plan: {
          ...plan,
          sourceAudio: plan.sourceAudio?.filter((setting) =>
            plan.cuts.some(
              (cut) => cut.sourceId === setting.sourceId && cut.fingerprint === setting.fingerprint,
            ),
          ),
        },
      })
    },
    retry: false,
  })
  function publish(next: ClipProject) {
    cache.setQueryData([...projectsKey, 'detail', project.id], next)
    void cache.invalidateQueries({ queryKey: projectsKey })
  }
  function adopt(next: ClipProject, clearHistory = false) {
    if (!next.editing) return
    dispatch({
      type: 'adopt',
      plan: next.editing.plan,
      clearHistory,
      regionsFrom: next.editing.plan,
    })
    setBaseline(clipDraftKey(next.editing.plan))
    setRevision(next.editPlanRevision)
    setRemoteConflict(false)
  }
  /** A newer saved plan under a draft the owner is still correcting. A slot, preset or
   *  enablement save moves only the region elements (CLIP-188): the draft takes them and keeps
   *  every other correction, rather than sending the old region words back as the owner's own.
   *  Anything else the server moved is a real conflict, left for the owner to resolve. */
  function rebase(next: ClipProject, local: ClipEditPlan, base: string) {
    if (!next.editing) return undefined
    const plan = rebaseClipRegions(local, JSON.parse(base) as ClipEditPlan, next.editing.plan)
    if (!plan) {
      setRemoteConflict(true)
      return undefined
    }
    dispatch({ type: 'adopt', plan, regionsFrom: next.editing.plan })
    setBaseline(clipDraftKey(next.editing.plan))
    setRevision(next.editPlanRevision)
    return plan
  }
  // Never let an older query response replace a newer accepted save or draft.
  if (
    !saving.current &&
    !mutation.isPending &&
    project.editing &&
    project.editPlanRevision > revision
  ) {
    if (!dirty) adopt(project)
    else if (!remoteConflict) rebase(project, draft, baseline)
  }
  /** Takes whatever the lane's earlier writes left in the cache before this draft is sent or
   *  counted as saved: a settings or slot save that landed a moment ago has not re-rendered
   *  this hook yet, and its revision is the one the next save must name. */
  function incorporateLatest() {
    const latest = cache.getQueryData<ClipProject>([...projectsKey, 'detail', project.id])
    const known = current.current
    if (!latest?.editing || latest.editPlanRevision <= known.revision || known.remoteConflict)
      return
    const saved = clipDraftKey(latest.editing.plan)
    if (!known.dirty) {
      adopt(latest)
      current.current = {
        ...known,
        draft: latest.editing.plan,
        baseline: saved,
        revision: latest.editPlanRevision,
      }
      return
    }
    const plan = rebase(latest, known.draft, known.baseline)
    current.current = plan
      ? {
          ...known,
          draft: plan,
          baseline: saved,
          revision: latest.editPlanRevision,
          dirty: clipDraftKey(plan) !== saved,
        }
      : { ...known, remoteConflict: true }
  }
  const change = (edit: TimelineEdit, group?: string) => {
    if ((!project.editing && edit.type !== 'sourceSound') || active) return
    if (mutation.error && appFailureFromConnect(mutation.error).reason !== 'CLIP_PLAN_CONFLICT')
      mutation.reset()
    dispatch({ type: 'edit', edit, group, at: Date.now() })
  }
  function saveOne(): Promise<number> {
    if (saving.current) return saving.current
    // The snapshot is taken INSIDE the lane: a slot or settings save queued ahead of this one
    // moves the revision this save has to name.
    const operation = serialClipWrite(project.id, async () => {
      incorporateLatest()
      const submitted = current.current
      if (submitted.remoteConflict) throw new Error('Conflicting clip draft')
      if (!submitted.dirty) return submitted.revision
      const accepted = JSON.parse(submitted.baseline) as ClipEditPlan
      const source = [...soundSources.current.values()].find(
        (source) =>
          clipSourceSound(submitted.draft, source, source.retainOriginalAudio) !==
          clipSourceSound(accepted, source, source.retainOriginalAudio),
      )
      if ((!submitted.valid && !source) || active) throw new Error('Invalid clip draft')
      const sound = source
        ? {
            ...source,
            retainOriginalAudio: clipSourceSound(
              submitted.draft,
              source,
              source.retainOriginalAudio,
            ),
          }
        : undefined
      const snapshot = copyClipPlan(submitted.draft)
      const key = clipDraftKey(snapshot)
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
          clipDraftKey(queued) === key
            ? acceptedPlan
            : acknowledgeClipCuts(queued, acceptedPlan, snapshot)
      }
      dispatch({ type: 'adopt', plan: queued, identitiesFrom: snapshot })
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
    }).finally(() => {
      saving.current = undefined
    })
    saving.current = operation
    return operation
  }
  // UI autosave keeps the draft on failure; a committing action must receive the failure.
  const save = () => flush().catch(() => undefined)
  async function flush(): Promise<number> {
    // Every write already in the lane lands first, and its revision is taken: a committing
    // action renders or finalizes the plan as the LAST of them left it.
    await serialClipWrite(project.id, async () => incorporateLatest())
    if (saving.current) await saving.current
    while (current.current.dirty) await saveOne()
    if (current.current.remoteConflict) throw new Error('Conflicting clip draft')
    return current.current.revision
  }
  const saveRef = useRef(save)
  useLayoutEffect(() => {
    saveRef.current = save
  })
  // No dialog guards leaving ② (CLIP-39): an edit still waiting for its autosave is sent when the
  // correction unmounts or the page is hidden, so leaving never takes it along. The send outlives
  // the component; only its answer is dropped.
  useEffect(() => {
    const leave = () => {
      if (current.current.dirty) void saveRef.current()
    }
    window.addEventListener('pagehide', leave)
    return () => {
      window.removeEventListener('pagehide', leave)
      leave()
    }
  }, [])
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
      const next = await projects.fetch(project.id)
      const batches = soundSources.current.size ? await sources.retained(project.id) : []
      let accepted = next.editing?.plan ?? { durationMs: 0, cuts: [] }
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
  const saved = useMemo(() => JSON.parse(baseline) as ClipEditPlan, [baseline])
  const audioPlan =
    mutation.error || remoteConflict
      ? { ...draft, sourceAudio: (JSON.parse(baseline) as ClipEditPlan).sourceAudio }
      : draft
  return {
    soundBatch,
    transientPlan,
    setTransientPlan,
    timelineBounds: (id: string): SourceBounds => {
      const cut = current.current.draft.cuts.find((c) => c.id === id)
      if (!cut) return { startMs: 0, endMs: 0 }
      const segment = project.observations?.sources
        .find((s) => s.source.id === cut.sourceId && s.source.fingerprint === cut.fingerprint)
        ?.segments.find(
          (s) => s.usability !== 'unusable' && s.startMs <= cut.startMs && cut.endMs <= s.endMs,
        )
      const accepted = project.editing?.plan.cuts.find((c) => c.id === id)
      return segment
        ? { startMs: segment.startMs, endMs: segment.endMs }
        : { startMs: accepted?.startMs ?? cut.startMs, endMs: accepted?.endMs ?? cut.endMs }
    },
    previewPlan: {
      ...(transientPlan ?? audioPlan),
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
    refreshCaptions: async () => {
      await flush()
      change({ type: 'refreshCaptions' })
    },
    splitCut: async (id: string, sourceMs: number) => {
      await flush()
      const plan = current.current.draft
      const item = timelineCuts(plan).find((c) => c.cut.id === id)
      const edit = item && splitAtOutput(plan, id, sourceToOutputMs(item, sourceMs), createCutId())
      if (!edit) throw new Error('Invalid split position')
      change(edit)
    },
    draft,
    /** The plan as the server last accepted it: what an unsaved edit is measured against. */
    saved,
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
