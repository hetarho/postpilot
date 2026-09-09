export const CLIP_RATIOS = ['vertical', 'horizontal', 'square'] as const
export type ClipRatio = (typeof CLIP_RATIOS)[number]
export const CLIP_PROJECT_LIMITS = {
  title: 100,
  answer: 500,
  minSeconds: 15,
  maxSeconds: 90,
} as const
export interface ClipProjectDraft {
  title: string
  videoTemplateId: string
  ratio: ClipRatio
  targetDurationMs: number
  answers: Array<{ label: string; text: string }>
}
export interface ClipProject extends ClipProjectDraft {
  id: string
  createdAt: string
  updatedAt: string
  editPlanRevision: number
  renderedPlanRevision: number
  result?: { contentType: string; bytes: number; durationMs: number; createdAt: string }
}
export interface ClipSourceMetadata {
  filename: string
  contentType: string
  bytes: number
  durationMs: number
  width: number
  height: number
  fingerprint: string
}
export interface ClipSourceBatch {
  id: string
  projectId: string
  state: 'uploading' | 'ready' | 'consuming' | 'cleanup_pending'
  expiresAt: string
  sources: Array<{
    id: string
    state: 'pending' | 'ready'
    actualBytes: number
    metadata: ClipSourceMetadata
  }>
}
export type ReadyClipBatch = ClipSourceBatch & { state: 'ready' }
export function emptyClipProject(): ClipProjectDraft {
  return { title: '', videoTemplateId: '', ratio: 'vertical', targetDurationMs: 30000, answers: [] }
}
export function projectDraft(value: ClipProjectDraft): ClipProjectDraft {
  return {
    title: value.title,
    videoTemplateId: value.videoTemplateId,
    ratio: value.ratio,
    targetDurationMs: value.targetDurationMs,
    answers: value.answers.map((a) => ({ ...a })),
  }
}
export function normalizeClipProject(value: ClipProjectDraft): ClipProjectDraft {
  return { ...projectDraft(value), title: value.title.trim() }
}
export function validClipProject(
  value: ClipProjectDraft,
  fields: readonly { label: string }[] | undefined,
): boolean {
  const length = (s: string) => Array.from(s).length
  return (
    !!value.title.trim() &&
    length(value.title.trim()) <= CLIP_PROJECT_LIMITS.title &&
    !!value.videoTemplateId &&
    !!fields &&
    CLIP_RATIOS.includes(value.ratio) &&
    Number.isInteger(value.targetDurationMs) &&
    value.targetDurationMs >= CLIP_PROJECT_LIMITS.minSeconds * 1000 &&
    value.targetDurationMs <= CLIP_PROJECT_LIMITS.maxSeconds * 1000 &&
    value.answers.every((a) => length(a.text) <= CLIP_PROJECT_LIMITS.answer) &&
    fields.every((field) => value.answers.some((a) => a.label === field.label && !!a.text.trim()))
  )
}
