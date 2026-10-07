export const WRITING_VOICE_CANDIDATE_COUNT = 8
export const WRITING_VOICE_CANDIDATE_COUNTS = [2, 4, 8, 16] as const
export type WritingVoiceCandidateCount = (typeof WRITING_VOICE_CANDIDATE_COUNTS)[number]
export interface WritingVoiceCandidate {
  id: string
  name: string
  description: string
  sample: string
}
export interface WritingVoiceCandidateBatch {
  jobId: string
  resultJobId: string
  candidates: WritingVoiceCandidate[]
}
export interface WritingVoiceCandidateEstimate {
  credits: number | undefined
  free: boolean
}
export function completeCandidateBatch(
  candidates: readonly WritingVoiceCandidate[],
  expectedCount?: WritingVoiceCandidateCount,
) {
  return (
    WRITING_VOICE_CANDIDATE_COUNTS.some((count) => count === candidates.length) &&
    (expectedCount === undefined || candidates.length === expectedCount) &&
    new Set(candidates.map((candidate) => candidate.id)).size === candidates.length &&
    candidates.every(
      (candidate) => candidate.id && candidate.name && candidate.description && candidate.sample,
    )
  )
}
