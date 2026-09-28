import type {
  ClipProjectRegion,
  ClipProjectRegions,
  ClipRegionEdit,
  ClipRegionSlot,
} from '@/entities/clip-project'
import { clipRegionSlots, type ClipRegionKind } from '@/entities/clip-design'

export const CLIP_REGION_KINDS = ['intro', 'outro'] as const satisfies readonly ClipRegionKind[]

/** What the owner changed in one slot and has not seen saved yet. */
export interface ClipSlotEdit {
  instruction?: string
  text?: string
}

/** One region's unsaved edits: its switch and its slots' fields, by slot id. */
export interface ClipRegionEdits {
  enabled?: boolean
  slots: Record<string, ClipSlotEdit>
}

export type ClipRegionsEdits = Record<ClipRegionKind, ClipRegionEdits>

export const noRegionEdits = (): ClipRegionsEdits => ({
  intro: { slots: {} },
  outro: { slots: {} },
})

export const hasRegionEdits = (edits: ClipRegionsEdits) =>
  CLIP_REGION_KINDS.some(
    (kind) => edits[kind].enabled !== undefined || Object.keys(edits[kind].slots).length > 0,
  )

/** The regions as the owner sees them: the server's, with every unsaved edit over it. A slot
 *  whose final text the owner typed is theirs from that keystroke (CLIP-186). */
export function withRegionEdits(
  server: ClipProjectRegions,
  edits: ClipRegionsEdits,
): ClipProjectRegions {
  const region = (value: ClipProjectRegion, edit: ClipRegionEdits): ClipProjectRegion => ({
    enabled: edit.enabled ?? value.enabled,
    slots: value.slots.map((slot) => {
      const change = edit.slots[slot.id]
      if (!change) return slot
      return {
        ...slot,
        instruction: change.instruction ?? slot.instruction,
        instructionEdited: slot.instructionEdited || change.instruction !== undefined,
        text: change.text ?? slot.text,
        ownerFixed: slot.ownerFixed || change.text !== undefined,
        bound: change.text !== undefined ? false : slot.bound,
      }
    }),
  })
  return {
    ...server,
    intro: region(server.intro, edits.intro),
    outro: region(server.outro, edits.outro),
  }
}

/** The presence-aware patch one region's edits make: a field left out stays as it is, while
 *  false and an empty text are real edits (T450). Undefined when nothing changed. */
export function regionPatch(edits: ClipRegionEdits): ClipRegionEdit | undefined {
  const slots = Object.entries(edits.slots).map(([id, change]) => ({
    id,
    ...(change.instruction !== undefined ? { instruction: change.instruction } : {}),
    ...(change.text !== undefined ? { text: change.text } : {}),
  }))
  if (edits.enabled === undefined && slots.length === 0) return undefined
  return { ...(edits.enabled !== undefined ? { enabled: edits.enabled } : {}), slots }
}

/** The edits a save response has not caught up with: every field whose saved value is not
 *  yet the one the owner typed. What was saved — or overwritten by something newer the owner
 *  sees — drops out, so the next save sends only what is still the owner's alone. */
export function unsavedRegionEdits(
  server: ClipProjectRegions,
  edits: ClipRegionsEdits,
): ClipRegionsEdits {
  const out = noRegionEdits()
  for (const kind of CLIP_REGION_KINDS) {
    const saved = server[kind]
    const edit = edits[kind]
    if (edit.enabled !== undefined && edit.enabled !== saved.enabled)
      out[kind].enabled = edit.enabled
    for (const [id, change] of Object.entries(edit.slots)) {
      const slot = saved.slots.find((s) => s.id === id)
      if (!slot) continue
      const left: ClipSlotEdit = {}
      if (change.instruction !== undefined && change.instruction !== slot.instruction)
        left.instruction = change.instruction
      if (change.text !== undefined && (change.text !== slot.text || !slot.ownerFixed))
        left.text = change.text
      if (left.instruction !== undefined || left.text !== undefined) out[kind].slots[id] = left
    }
  }
  return out
}

/** How many slots the chosen preset draws (CLIP-147); the rest are unused drafts. */
export const regionCapacity = (kind: ClipRegionKind, preset: string) =>
  clipRegionSlots(kind, preset).length

/** What one slot holds, as the storyline block names it (CLIP-186, CLIP-187). */
export type ClipSlotState = 'owner' | 'blank' | 'bound' | 'written' | 'awaiting'

export function slotState(slot: ClipRegionSlot): ClipSlotState {
  if (slot.bound) return 'bound'
  if (slot.ownerFixed) return slot.text.trim() ? 'owner' : 'blank'
  return slot.text.trim() ? 'written' : 'awaiting'
}

/** Whether an enabled region draws anything at all: an enabled block with no drawable text is
 *  unresolved rather than included (CLIP-187). A generated text its slot cannot fit is kept but
 *  not drawn (CDS-77); the owner's own and an answer's are drawn, or refused, as they are. */
export function regionDraws(
  region: ClipProjectRegion,
  capacity: number,
  fits: (index: number, text: string) => boolean,
) {
  return (
    region.enabled &&
    region.slots
      .slice(0, capacity)
      .some(
        (slot, index) =>
          slot.text.trim() !== '' && (slot.ownerFixed || slot.bound || fits(index, slot.text)),
      )
  )
}

/** `edits` with one slot's field changed. */
export function withSlotEdit(
  edits: ClipRegionsEdits,
  kind: ClipRegionKind,
  id: string,
  change: ClipSlotEdit,
): ClipRegionsEdits {
  const slot = { ...edits[kind].slots[id], ...change }
  return { ...edits, [kind]: { ...edits[kind], slots: { ...edits[kind].slots, [id]: slot } } }
}

/** What of `edits` may go out: an active slot's final text its slot cannot draw stays in the
 *  field with its error and is not sent (CLIP-189, CDS-64); everything else goes. */
export function sendableRegionEdits(
  server: ClipProjectRegions,
  edits: ClipRegionsEdits,
  capacity: (kind: ClipRegionKind) => number,
  fits: (kind: ClipRegionKind, index: number, text: string) => boolean,
): ClipRegionsEdits {
  const out = noRegionEdits()
  for (const kind of CLIP_REGION_KINDS) {
    const enabled = edits[kind].enabled ?? server[kind].enabled
    if (edits[kind].enabled !== undefined) out[kind].enabled = edits[kind].enabled
    for (const [id, change] of Object.entries(edits[kind].slots)) {
      const index = server[kind].slots.findIndex((slot) => slot.id === id)
      const unfit =
        change.text !== undefined &&
        enabled &&
        index >= 0 &&
        index < capacity(kind) &&
        !fits(kind, index, change.text)
      const kept: ClipSlotEdit = unfit ? { instruction: change.instruction } : change
      if (kept.instruction !== undefined || kept.text !== undefined) out[kind].slots[id] = kept
    }
  }
  return out
}
