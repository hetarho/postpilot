import { clipDraftKey } from './draft-key'
import type { ClipEditPlan, ClipEditableText } from './edit-plan'

/** The role each region draws in. The project's slots own an intro's and an outro's words
 *  (CLIP-186), and the plan draws them as ONE element per enabled region (CLIP-188). */
export const CLIP_REGION_ROLES = { intro: 'hook', outro: 'ending' } as const
const ROLES: readonly string[] = Object.values(CLIP_REGION_ROLES)

/** The element a project's slots of `kind` are drawn as, one row per active slot. */
export const clipRegionElementId = (kind: keyof typeof CLIP_REGION_ROLES) => `project-${kind}`

/** The words a plan draws for a region, one per active slot of its preset, blank ones kept;
 *  undefined when the plan draws no element for it. */
export function clipRegionRows(plan: ClipEditPlan, kind: keyof typeof CLIP_REGION_ROLES) {
  const element = plan.elements?.find((t) => t.instanceId === clipRegionElementId(kind))
  return element?.role === CLIP_REGION_ROLES[kind] ? element.rows.map((r) => r.text) : undefined
}

const roleOf = (plan: ClipEditPlan, role: string) =>
  (plan.elements ?? []).filter((t) => t.role === role)
const keyOf = (elements: ClipEditableText[]) => clipDraftKey({ durationMs: 0, cuts: [], elements })

/** `elements` with the role's entries replaced by `next`, where the role stood — or, where it
 *  did not, where the server put it. */
function placeRole(
  elements: ClipEditableText[],
  role: string,
  next: ClipEditableText[],
  server: ClipEditPlan,
) {
  const kept = elements.filter((t) => t.role !== role)
  const at = elements.findIndex((t) => t.role === role)
  const index =
    at >= 0
      ? at
      : Math.min(
          Math.max(
            0,
            (server.elements ?? []).findIndex((t) => t.role === role),
          ),
          kept.length,
        )
  return [...kept.slice(0, index), ...next, ...kept.slice(index)]
}

/** Both drafts changed the same region: they join when the local one changed nothing but rows
 *  and no row was changed differently by both. */
function mergeRows(
  local: ClipEditableText[],
  base: ClipEditableText[],
  server: ClipEditableText[],
) {
  const [l, b, s] = [local[0], base[0], server[0]]
  if (local.length !== 1 || base.length !== 1 || server.length !== 1) return undefined
  if (l.instanceId !== b.instanceId || s.instanceId !== b.instanceId) return undefined
  if (l.rows.length !== b.rows.length || s.rows.length !== b.rows.length) return undefined
  const bare = (t: ClipEditableText) => ({
    ...t,
    rows: [],
    staleEvidence: undefined,
    evidenceReviewed: undefined,
  })
  if (keyOf([bare(l)]) !== keyOf([bare(b)])) return undefined
  const rows = s.rows.map((row, i) => {
    const typed = l.rows[i].text !== b.rows[i].text
    return typed ? { ...row, text: l.rows[i].text } : row
  })
  const clash = s.rows.some(
    (row, i) =>
      l.rows[i].text !== b.rows[i].text &&
      row.text !== b.rows[i].text &&
      row.text !== l.rows[i].text,
  )
  return clash ? undefined : [{ ...s, rows }]
}

/** A draft typed over `base`, carried onto `server` when what the server moved is the region
 *  elements and nothing else. Slot, preset and enablement saves project into the plan
 *  (CLIP-188) while the owner may be correcting anything else, and those corrections must
 *  survive the new revision without writing the old region words back over the new ones. A
 *  region the server left alone keeps the draft's; one the draft left alone takes the server's;
 *  one both changed joins row by row. Undefined when they cannot be joined, or when the new
 *  revision is not a region projection at all — a real conflict to recover from. */
export function rebaseClipRegions(
  local: ClipEditPlan,
  base: ClipEditPlan,
  server: ClipEditPlan,
): ClipEditPlan | undefined {
  const rest = (plan: ClipEditPlan) =>
    clipDraftKey({
      ...plan,
      elements: (plan.elements ?? []).filter((t) => !ROLES.includes(t.role)),
    })
  if (rest(server) !== rest(base)) return undefined
  let merged = local
  let moved = false
  for (const role of ROLES) {
    const [l, b, s] = [roleOf(local, role), roleOf(base, role), roleOf(server, role)]
    if (keyOf(s) === keyOf(b)) continue
    const next = keyOf(l) === keyOf(b) ? s : mergeRows(l, b, s)
    if (!next) return undefined
    merged = { ...merged, elements: placeRole(merged.elements ?? [], role, next, server) }
    moved = true
  }
  return moved ? merged : undefined
}

/** `plan` with every region element the server now holds in place of its own: the history a
 *  draft can step back through follows the slots too, so an undo never restores region words
 *  the slots have moved past (CLIP-188). */
export function withClipRegionsOf(plan: ClipEditPlan, server: ClipEditPlan): ClipEditPlan {
  let merged = plan
  for (const role of ROLES) {
    const next = roleOf(server, role)
    if (keyOf(roleOf(merged, role)) === keyOf(next)) continue
    merged = { ...merged, elements: placeRole(merged.elements ?? [], role, next, server) }
  }
  return merged
}
