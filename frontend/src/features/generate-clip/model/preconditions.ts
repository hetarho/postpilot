import type { ReadyClipBatch } from '@/entities/clip-project'
import { sameRef, type StageSelectionState } from '@/entities/model-catalog'

export function clipModelsReady(observe: StageSelectionState, write: StageSelectionState) {
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
  return !!o && !!w && !o.disabled && !w.disabled && o.vision && o.videoInput
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
