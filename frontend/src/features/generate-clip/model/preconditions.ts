import {
  clipEligibilityOf,
  projectDraft,
  type ClipEligibilityStatus,
  type ClipModelEligibility,
  type ClipProject,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import type { ModelRef } from '@/entities/model-catalog'
import { sameRef, type StageSelectionState } from '@/entities/model-catalog'

/** The live eligibility list as the page holds it: loading, failed, or the rows. */
export type ClipEligibilityState =
  | { kind: 'loading' }
  | { kind: 'failed' }
  | { kind: 'ready'; rows: readonly ClipModelEligibility[] }

/** The selected observe model's status under the live list, or undefined while the list is
 *  not ready or does not resolve the model. Only `'eligible'` lets a quote or a start go. */
export function selectedClipStatus(
  eligibility: ClipEligibilityState,
  observe: ModelRef | null,
): ClipEligibilityStatus | undefined {
  if (!observe || eligibility.kind !== 'ready') return undefined
  return clipEligibilityOf(eligibility.rows, observe)
}

/** Both stages chosen and usable, and the observe model ELIGIBLE under T111's live answer.
 *  `inlineStaticVideo` is a capability badge and gates nothing here (CLIP-30). */
export function clipModelsReady(
  observe: StageSelectionState,
  write: StageSelectionState,
  eligibility: ClipEligibilityState,
) {
  if (
    observe.isPending ||
    write.isPending ||
    observe.isError ||
    write.isError ||
    !observe.selected ||
    !write.selected
  )
    return false
  const observeRef = observe.selected
  const writeRef = write.selected
  const o = observe.models.find((m) => sameRef(m.ref, observeRef))
  const w = write.models.find((m) => sameRef(m.ref, writeRef))
  return (
    !!o &&
    !!w &&
    !o.disabled &&
    !w.disabled &&
    selectedClipStatus(eligibility, observeRef) === 'eligible'
  )
}

/** What a quote was read for. The observe model's eligibility status is part of it, so a
 *  status change invalidates a displayed quote the same way a model change does. */
export function clipQuoteBinding(
  project: ClipProject,
  batch: ReadyClipBatch,
  observe: ModelRef,
  write: ModelRef,
  status: ClipEligibilityStatus | undefined,
  /** 다시 관찰하기's chosen sources: a different choice is a different quote. */
  reanalyze?: readonly string[],
) {
  return JSON.stringify([
    project.id,
    projectDraft(project),
    project.updatedAt,
    batch.id,
    batch.sources,
    observe,
    write,
    status ?? null,
    ...(reanalyze ? [[...reanalyze].sort()] : []),
  ])
}

/** Whether a selected source has no stored observation of its current footage — 관찰하기 then
 *  analyzes it, and otherwise ① moves on to ② without a call (CLIP-234, CLIP-40). A source is
 *  observed only while the observation was made of the same file (identity and fingerprint). */
export function lacksClipObservation(
  sources: ReadonlyArray<{ sourceId: string; fingerprint: string }>,
  observations: ClipProject['observations'],
): boolean {
  const observed = new Set(
    (observations?.status === 'available' ? observations.sources : []).map((entry) =>
      JSON.stringify([entry.source.id, entry.source.fingerprint]),
    ),
  )
  return sources.some(
    (source) => !observed.has(JSON.stringify([source.sourceId, source.fingerprint])),
  )
}

export function readyClipBatch(batch: ReadyClipBatch | undefined, projectId: string, now: number) {
  return (
    !!batch &&
    batch.projectId === projectId &&
    batch.state === 'ready' &&
    Date.parse(batch.expiresAt) > now &&
    batch.sources.length > 0 &&
    batch.sources.every((s) => s.state === 'ready' && s.actualBytes === s.metadata.bytes)
  )
}

/** A saved edit may use only part of the retained batch. Match the server's
 * render admission: every used fingerprint must be present, and retained
 * fingerprints must remain unique. */
export function renderClipSourcesAvailable(
  batch: ReadyClipBatch,
  usedFingerprints: readonly string[],
) {
  const retained = new Set(batch.sources.map((source) => source.metadata.fingerprint))
  return (
    retained.size === batch.sources.length &&
    usedFingerprints.every((fingerprint) => retained.has(fingerprint))
  )
}
