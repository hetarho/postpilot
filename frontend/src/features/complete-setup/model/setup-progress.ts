import { SETUP_PROGRESS_PREFIX, SETUP_PROGRESS_VERSION } from '../config'
import {
  emptySetupProgress,
  safeSetupTarget,
  SETUP_FORMS,
  type SetupProgress,
  type SetupStep,
} from './setup-machine'
const memoryProgress = new Map<string, { progress: SetupProgress; fallback: boolean }>()
export function resetSetupProgressMemory(): void {
  memoryProgress.clear()
}
export interface SetupStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}
export function setupProgressKey(ownerId: string) {
  return SETUP_PROGRESS_PREFIX + encodeURIComponent(ownerId)
}
export function browserSetupStorage(): SetupStorage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}
export function readSetupProgress(
  ownerId: string,
  storage: SetupStorage | null = browserSetupStorage(),
): SetupProgress {
  if (!ownerId) return emptySetupProgress()
  try {
    const raw = storage?.getItem(setupProgressKey(ownerId)) ?? null
    if (raw === null) {
      const cached = memoryProgress.get(ownerId)
      return !storage || cached?.fallback
        ? (cached?.progress ?? emptySetupProgress())
        : emptySetupProgress()
    }
    let parsed: unknown
    try {
      parsed = JSON.parse(raw)
    } catch {
      memoryProgress.delete(ownerId)
      return emptySetupProgress()
    }
    if (!parsed || typeof parsed !== 'object') return emptySetupProgress()
    const p = parsed as Record<string, unknown>
    const steps: readonly string[] = ['welcome', ...SETUP_FORMS, 'ready']
    if (
      p.version !== SETUP_PROGRESS_VERSION ||
      p.ownerId !== ownerId ||
      typeof p.completed !== 'boolean' ||
      !Array.isArray(p.skipped) ||
      !p.skipped.every((s) => SETUP_FORMS.includes(s)) ||
      typeof p.resume !== 'string' ||
      !steps.includes(p.resume)
    )
      return emptySetupProgress()
    return {
      completed: p.completed,
      skipped: [...new Set(p.skipped)],
      resume: p.resume as SetupStep,
      target: safeSetupTarget(p.target),
    }
  } catch {
    return memoryProgress.get(ownerId)?.progress ?? emptySetupProgress()
  }
}
export function writeSetupProgress(
  ownerId: string,
  progress: SetupProgress,
  storage: SetupStorage | null = browserSetupStorage(),
): void {
  if (!ownerId) return
  const normalized = {
    completed: progress.completed,
    skipped: [...progress.skipped],
    resume: progress.resume,
    target: safeSetupTarget(progress.target),
  }
  memoryProgress.set(ownerId, { progress: normalized, fallback: true })
  try {
    if (!storage) return
    storage.setItem(
      setupProgressKey(ownerId),
      JSON.stringify({ version: SETUP_PROGRESS_VERSION, ownerId, ...normalized }),
    )
    memoryProgress.set(ownerId, { progress: normalized, fallback: false })
  } catch {
    /* Keep progress across navigation when browser storage rejects writes. */
  }
}
