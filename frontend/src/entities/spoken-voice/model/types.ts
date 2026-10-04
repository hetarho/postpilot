/** Sound identities never refer to a writing-style Voice record. */
export interface SpokenProfile {
  id: string
  revision: bigint
  providerId: string
  designModelId: string
  speechModelId: string
  designLabel: string
  speechLabel: string
  grade: 'free' | 'value' | 'balanced' | 'premium' | 'top' | null
  descriptionMax: number
  previewMax: number
  speechMax: number
  outputFormat: string
}
export interface SpokenCandidate {
  id: string
  assetId: string
  durationMs: number
  auditionedAt: string
}
export type SpokenPhase = 'editing' | 'candidates' | 'selected' | 'confirmed'
export interface SpokenDraft {
  id: string
  revision: bigint
  name: string
  description: string
  previewText: string
  profile: SpokenProfile
  phase: SpokenPhase
  candidates: SpokenCandidate[]
  selectedCandidateId: string
  confirmedVoiceId: string
  qualificationSessionId: string
  createdAt: string
  updatedAt: string
}
export interface SpokenVoice {
  id: string
  revision: bigint
  name: string
  description: string
  previewText: string
  profile: SpokenProfile
  sampleAssetId: string
  sampleDurationMs: number
  createdAt: string
  removedAt: string
}
export interface SpokenDraftInput {
  name: string
  description: string
  previewText: string
  profileId: string
  profileRevision: bigint
  qualificationSessionId: string
}

export type SpokenWorkKind = 'voice_design' | 'voice_confirm' | 'voice_reuse_probe'
export type SpokenOperationState =
  | 'reserved'
  | 'queued'
  | 'claimed'
  | 'received'
  | 'published'
  | 'failed'
  | 'unresolved'
  | 'cancelled'
export interface SpokenOperation {
  id: string
  kind: SpokenWorkKind
  state: SpokenOperationState
  jobId: string
  draftId: string
  voiceId: string
  candidateId: string
  resultId: string
  failureReason: string
}
export interface SpokenWorkQuote {
  id: string
  maximumCredits: number
  expiresAt: string
  approvalRequired: boolean
  existingVoiceId: string
}
export interface SpokenWorkInput {
  kind: 'voice_design' | 'voice_confirm'
  draftId: string
  revision: bigint
  candidateId?: string
}
export const spokenOperationActive = (s: SpokenOperationState) =>
  s === 'reserved' || s === 'queued' || s === 'claimed'
