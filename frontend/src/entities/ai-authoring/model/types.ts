export const AUTHORING_KINDS = [
  'post-template',
  'video-template',
  'post-guideline',
  'video-guideline',
  'writing-voice',
] as const
export type AuthoringKind = (typeof AUTHORING_KINDS)[number]
export type AuthoringMode = 'recommend' | 'refine'
export const AUTHORING_PHASES = [
  'choosing',
  'editing',
  'generating',
  'refining',
  'saving',
  'saved',
  'failed',
] as const
export type AuthoringPhase = (typeof AUTHORING_PHASES)[number]
export interface AuthoringScope {
  ownerId: string
  kind: AuthoringKind
  targetId?: string
}
export const AUTHORING_CANDIDATE_COUNTS = [
  1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
] as const
export type AuthoringCandidateCount = (typeof AUTHORING_CANDIDATE_COUNTS)[number]
export type AuthoringDraftState = 'valid' | 'incomplete' | 'invalid'
export interface AuthoringArtifact {
  targetLength?: string
  tagCount?: string
  scope?: string
  templateIds?: string[]
  fields?: string[]
  builderState?: string
  revision?: number
  id: string
  name: string
  description: string
  body: string
  titleArea: string
}
export interface AuthoringTurn {
  id: string
  request: string
  reply: string
  jobId: string
  status: string
}
export interface AuthoringSavedRef {
  outcome?: 'created' | 'updated'
  kind: AuthoringKind
  id: string
  name: string
}
export interface AuthoringSession {
  workingSource?: AuthoringArtifact
  savedBaseline?: AuthoringArtifact
  draftState?: AuthoringDraftState
  hasUnpublishedChanges?: boolean
  savedAvailable?: boolean
  candidateCount?: AuthoringCandidateCount
  id: string
  kind: AuthoringKind
  revision: number
  phase: AuthoringPhase
  targetId: string
  targetVersion: string
  candidates: AuthoringArtifact[]
  selected?: AuthoringArtifact
  turns: AuthoringTurn[]
  activeJobId: string
  saved?: AuthoringSavedRef
  failureReason: string
  pendingRequest: string
}
export interface AuthoringModelRef {
  providerId: string
  modelId: string
}
export interface AuthoringEstimate {
  free: boolean
  credits?: number
}
export interface AuthoringStart {
  candidateCount?: AuthoringCandidateCount
  sessionId: string
  expectedRevision: number
  requestId: string
  mode: AuthoringMode
  prompt: string
  writeModel: AuthoringModelRef
}
export const AUTHORING_REFERENCE_POST_MAX_CHARS = 12000
export const AUTHORING_BUILDER_STATE_MAX_CHARS = 40000
export const AUTHORING_NAME_SUMMARY_MAX_CHARS = 80
export const AUTHORING_SUGGESTION_COUNT = 8
export const AUTHORING_MESSAGE_MAX_CHARS = 2000
export const AUTHORING_MAX_EXCHANGES = 20
export function authoringScopeKey(scope: AuthoringScope): string {
  return JSON.stringify([scope.ownerId, scope.kind, scope.targetId ?? ''])
}
export function authoringSessionBusy(session: AuthoringSession | undefined): boolean {
  return (
    !!session &&
    (session.activeJobId !== '' || ['generating', 'refining', 'saving'].includes(session.phase))
  )
}
export function completedAuthoringExchanges(session: AuthoringSession | undefined): number {
  return session?.turns.filter((turn) => turn.status === 'done').length ?? 0
}

export interface AuthoringSummary {
  sessionId: string
  kind: AuthoringKind
  targetId: string
  displayName: string
  revision: number
  savedAvailable: boolean
  hasUnpublishedChanges: boolean
  activeJobId: string
  publicationPending: boolean
  targetConflict: boolean
  draftState: AuthoringDraftState
  lastPublication?: AuthoringSavedRef
  updatedAt: string
}
