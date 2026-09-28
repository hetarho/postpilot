import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  serialClipWrite,
  useClipProjectCalls,
  useClipProjectsKey,
  useSaveClipRegions,
  type ClipNotice,
  type ClipProject,
  type ClipProjectRegions,
} from '@/entities/clip-project'
import {
  CLIP_DEFAULT_REGION_PRESETS,
  clipRegionSlotFit,
  type ClipRegionKind,
  type ClipRegionRatio,
} from '@/entities/clip-design'
import type { AppFailure } from '@/shared/api'
import { SAVE_STATUS_SETTLED_MS } from '@/shared/config'
import { resolveSaveStatus, SAVE_STATUS_LABEL_KEYS } from '@/shared/lib'
import {
  CLIP_REGION_KINDS,
  noRegionEdits,
  regionCapacity,
  regionDraws,
  regionPatch,
  sendableRegionEdits,
  unsavedRegionEdits,
  withRegionEdits,
  withSlotEdit,
  type ClipRegionsEdits,
  type ClipSlotEdit,
} from './region-draft'
import {
  clipRegionsFailure,
  clipRegionsState,
  flushClipRegions,
  peekClipRegions,
  queueClipRegions,
  regionConflict,
  subscribeClipRegions,
} from './region-queue'

/** The plan draft's side of a region's words, supplied by the workspace that holds both: while
 *  the edit plan draws a region, its rows ARE the active slots' final text (CLIP-188), and a
 *  slot typed here is the same edit ②'s text controls make — one draft, one queue, and the
 *  draft preview follows each keystroke. */
export interface ClipRegionPlan {
  /** The rows the plan draft draws for a region, one per active slot, or undefined. */
  rows: (kind: ClipRegionKind) => readonly string[] | undefined
  /** The same rows as the server last accepted them. */
  savedRows: (kind: ClipRegionKind) => readonly string[] | undefined
  setRow: (kind: ClipRegionKind, index: number, text: string) => void
}

const presetOf = (project: ClipProject, kind: ClipRegionKind) =>
  (kind === 'intro' ? project.introPreset : project.outroPreset) ||
  CLIP_DEFAULT_REGION_PRESETS[kind]

/** ②'s intro and outro slots (CLIP-179, CLIP-186, CLIP-188): the server's regions with the
 *  owner's unsaved edits over them, the words the plan draft already draws, each field's
 *  refusal, and a queue that saves them. Enablement, instructions, unused words and a region
 *  the plan does not draw save as region edits; a drawn slot's words go into the plan draft,
 *  so the storyline block and the correction never hold two copies of one text. */
export function useClipRegionsEditor(
  ownerId: string,
  project: ClipProject,
  {
    plan,
    failures = [],
    readOnly,
    before,
  }: {
    plan?: ClipRegionPlan
    /** Refusals another save answered, bound to a slot by its id (the settings', the plan's). */
    failures?: readonly (AppFailure | undefined)[]
    readOnly: boolean
    /** What lands before a region save goes out: the settings, so a preset chosen before a
     *  switch reaches the server before it does (CLIP-111). */
    before?: () => Promise<unknown>
  },
) {
  const { t } = useTranslation('clips')
  const { t: common } = useTranslation('common')
  const cache = useQueryClient()
  const projectsKey = useClipProjectsKey(ownerId)
  const calls = useClipProjectCalls()
  const mutation = useSaveClipRegions(ownerId)
  const server = project.regions
  const [edits, setEditsState] = useState<ClipRegionsEdits>(() => {
    const owed = peekClipRegions(project.id)
    return owed && server ? unsavedRegionEdits(server, owed) : noRegionEdits()
  })
  const editsRef = useRef(edits)
  const setEdits = (next: ClipRegionsEdits) => {
    editsRef.current = next
    setEditsState(next)
  }
  const latest = useRef({ project, before })
  useLayoutEffect(() => {
    latest.current = { project, before }
  })

  const fits = (p: ClipProject, kind: ClipRegionKind, index: number, text: string) =>
    clipRegionSlotFit(kind, presetOf(p, kind), p.ratio as ClipRegionRatio, index, text) === ''
  const capacityOf = (p: ClipProject) => (kind: ClipRegionKind) =>
    regionCapacity(kind, presetOf(p, kind))

  /** One save of what is still owed, run in the project's write lane so it names the revision
   *  the writes before it left. A conflict is another tab's save: the patch carries only the
   *  fields the owner changed here, so it goes again once over the revision that won. */
  const send = async () => {
    await latest.current.before?.().catch(() => undefined)
    await serialClipWrite(project.id, async () => {
      const detail = [...projectsKey, 'detail', project.id]
      for (let attempt = 0; ; attempt++) {
        const now = cache.getQueryData<ClipProject>(detail) ?? latest.current.project
        if (!now.regions) return
        const owed = sendableRegionEdits(
          now.regions,
          unsavedRegionEdits(now.regions, editsRef.current),
          capacityOf(now),
          (kind, index, text) => fits(now, kind, index, text),
        )
        const introRegion = regionPatch(owed.intro)
        const outroRegion = regionPatch(owed.outro)
        if (!introRegion && !outroRegion) return
        try {
          const saved = await mutation.mutateAsync({
            id: project.id,
            expectedRegionRevision: now.regions.revision,
            ...(introRegion ? { introRegion } : {}),
            ...(outroRegion ? { outroRegion } : {}),
          })
          // What the answer holds is no longer the owner's alone; what they typed during the
          // flight, and a text held back as unfit, still is.
          if (saved.regions) setEdits(unsavedRegionEdits(saved.regions, editsRef.current))
          return
        } catch (error) {
          if (attempt > 0 || !regionConflict(error)) throw error
          cache.setQueryData(detail, await calls.fetch(project.id))
        }
      }
    })
  }
  const change = (next: ClipRegionsEdits) => {
    setEdits(next)
    queueClipRegions(project.id, next, send)
  }

  const capacity = {
    intro: regionCapacity('intro', presetOf(project, 'intro')),
    outro: regionCapacity('outro', presetOf(project, 'outro')),
  }
  const drawn = (kind: ClipRegionKind, shown: ClipProjectRegions) => {
    const rows = plan?.rows(kind)
    return shown[kind].enabled && rows?.length === capacity[kind] ? rows : undefined
  }
  let regions: ClipProjectRegions | undefined
  if (server) {
    regions = withRegionEdits(server, edits)
    for (const kind of CLIP_REGION_KINDS) {
      const rows = drawn(kind, regions)
      const saved = plan?.savedRows(kind)
      if (!rows) continue
      // A row the plan draft changed and has not saved is the owner's typing (CLIP-186); a row
      // it left alone shows the slot's own words, which keep a generated text the slot cannot
      // draw (CDS-77).
      regions[kind] = {
        ...regions[kind],
        slots: regions[kind].slots.map((slot, index) =>
          index < rows.length &&
          edits[kind].slots[slot.id]?.text === undefined &&
          rows[index] !== saved?.[index]
            ? { ...slot, text: rows[index], ownerFixed: true, bound: false }
            : slot,
        ),
      }
    }
  }

  const message = (reason: string | undefined) =>
    reason === 'copy_limit' || reason === 'unsupported_glyph'
      ? t(`regions.error.${reason}`)
      : t('regions.error.invalid')
  const errors: Record<string, string> = {}
  const drawable = { intro: false, outro: false }
  if (regions) {
    for (const kind of CLIP_REGION_KINDS) {
      const region = regions[kind]
      drawable[kind] = regionDraws(region, capacity[kind], (index, text) =>
        fits(project, kind, index, text),
      )
      if (!region.enabled) continue
      region.slots.slice(0, capacity[kind]).forEach((slot, index) => {
        if (!slot.ownerFixed && !slot.bound) return
        const reason = clipRegionSlotFit(
          kind,
          presetOf(project, kind),
          project.ratio as ClipRegionRatio,
          index,
          slot.text,
        )
        if (reason) errors[slot.id] = message(reason)
      })
    }
  }
  const invalid = Object.keys(errors).length > 0
  const slotIds = new Set(
    regions ? CLIP_REGION_KINDS.flatMap((kind) => regions[kind].slots.map((slot) => slot.id)) : [],
  )
  const subscribe = useCallback(
    (listener: () => void) => subscribeClipRegions(project.id, listener),
    [project.id],
  )
  const state = useSyncExternalStore(
    subscribe,
    () => clipRegionsState(project.id),
    () => 'idle' as const,
  )
  const failure = useSyncExternalStore(
    subscribe,
    () => clipRegionsFailure(project.id),
    () => undefined,
  )
  for (const refusal of [failure, ...failures]) {
    const id = refusal?.params.element_id
    if (refusal?.reason === 'CLIP_COMPOSITION_INVALID' && id && slotIds.has(id) && !errors[id])
      errors[id] = message(refusal.params.reason)
  }
  const invalidRef = useRef(invalid)
  useLayoutEffect(() => {
    invalidRef.current = invalid
  })

  // How long 저장했어요 stays on the one status line is presentation, as the settings' is.
  const [settled, setSettled] = useState(false)
  const [reported, setReported] = useState(state)
  if (reported !== state) {
    setReported(state)
    if (settled) setSettled(false)
  }
  useEffect(() => {
    if (state !== 'saved') return
    const timer = setTimeout(() => setSettled(true), SAVE_STATUS_SETTLED_MS)
    return () => clearTimeout(timer)
  }, [state])
  const resolved = resolveSaveStatus(state, settled)
  const label = SAVE_STATUS_LABEL_KEYS[resolved]

  const text = (kind: ClipRegionKind, id: string) =>
    regions?.[kind].slots.find((slot) => slot.id === id)?.text ?? ''
  /** Where a slot's final text goes: into the plan draft while the plan draws that slot, no
   *  region edit of its words is still owed and the slot can draw them; as a region edit
   *  otherwise, where words the slot cannot draw wait in the field with their error rather than
   *  reaching the preview or the plan (CLIP-189). */
  const setText = (next: ClipRegionsEdits, kind: ClipRegionKind, id: string, value: string) => {
    const index = server?.[kind].slots.findIndex((slot) => slot.id === id) ?? -1
    const rows = regions ? drawn(kind, regions) : undefined
    if (
      plan &&
      rows &&
      index >= 0 &&
      index < rows.length &&
      next[kind].slots[id]?.text === undefined &&
      fits(project, kind, index, value)
    ) {
      plan.setRow(kind, index, value)
      return next
    }
    return withSlotEdit(next, kind, id, { text: value })
  }
  const editable = !readOnly && !!server

  return {
    regions,
    capacity,
    presets: {
      intro: project.introPreset || CLIP_DEFAULT_REGION_PRESETS.intro,
      outro: project.outroPreset || CLIP_DEFAULT_REGION_PRESETS.outro,
    },
    drawable,
    errors,
    invalid,
    readOnly: readOnly || !server,
    /** The slot notices the region blocks show, so no other list repeats them. */
    slotIds,
    notices: (id: string): ClipNotice[] =>
      (project.notices ?? []).filter((notice) => notice.elementId === id),
    // Words held back as unfit are a save that cannot happen until they change, so the one
    // status line says so wherever the owner is (CLIP-38).
    status: invalid
      ? { state, failing: true, failure: undefined, label: t('regions.held') }
      : {
          state,
          failing: resolved === 'error' || resolved === 'refused',
          failure,
          label: label ? common(label) : '',
        },
    setEnabled: (kind: ClipRegionKind, enabled: boolean) => {
      if (!editable) return
      const current = editsRef.current
      change({ ...current, [kind]: { ...current[kind], enabled } })
    },
    setSlot: (kind: ClipRegionKind, id: string, value: ClipSlotEdit) => {
      if (!editable) return
      let next = editsRef.current
      if (value.instruction !== undefined)
        next = withSlotEdit(next, kind, id, { instruction: value.instruction })
      if (value.text !== undefined) next = setText(next, kind, id, value.text)
      if (next !== editsRef.current) change(next)
    },
    /** Moves unused words into an active slot and the slot's words into their place, so neither
     *  is lost (CLIP-189). */
    move: (kind: ClipRegionKind, from: string, to: string) => {
      if (!editable) return
      const moved = text(kind, from)
      const replaced = text(kind, to)
      change(
        withSlotEdit(setText(editsRef.current, kind, to, moved), kind, from, { text: replaced }),
      )
    },
    /** Sends what is owed now. A final text its slot cannot draw refuses the committing action
     *  and keeps its error, rather than going out without it (CLIP-188). */
    flush: async (failFast = false) => {
      if (invalidRef.current) throw new Error('Invalid clip regions')
      await flushClipRegions(project.id, failFast)
    },
  }
}

export type ClipRegionsEditor = ReturnType<typeof useClipRegionsEditor>
