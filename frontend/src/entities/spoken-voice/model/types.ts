/** Sound identities never refer to a writing-style Voice record. */
export interface SpokenProfile {
  id: string
  revision: bigint
  providerId: string
  designModelId: string
  speechModelId: string
  designLabel: string
  speechLabel: string
  grade: string
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
