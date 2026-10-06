export const WRITING_VOICE_CANDIDATE_COUNT = 8
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
export function completeCandidateBatch(candidates: readonly WritingVoiceCandidate[]) {
  return (
    candidates.length === WRITING_VOICE_CANDIDATE_COUNT &&
    new Set(candidates.map((candidate) => candidate.id)).size === WRITING_VOICE_CANDIDATE_COUNT &&
    candidates.every(
      (candidate) => candidate.id && candidate.name && candidate.description && candidate.sample,
    )
  )
}
