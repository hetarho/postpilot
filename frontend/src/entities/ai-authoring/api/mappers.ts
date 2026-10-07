import { createClient } from '@connectrpc/connect'
import {
  ConfigurationAuthoringService,
  ProtoConfigurationKind,
  ProtoAuthoringMode,
  ProtoAuthoringDraftState,
} from '@/shared/api'
import {
  AUTHORING_PHASES,
  AUTHORING_SUGGESTION_COUNT,
  AUTHORING_CANDIDATE_COUNTS,
  type AuthoringCandidateCount,
  type AuthoringDraftState,
  type AuthoringSummary,
  type AuthoringKind,
  type AuthoringMode,
  type AuthoringScope,
  type AuthoringSession,
  type AuthoringEstimate,
} from '../model/types'

type Client = ReturnType<typeof createClient<typeof ConfigurationAuthoringService>>
export type WireSession = NonNullable<Awaited<ReturnType<Client['getAuthoringSession']>>['session']>
const KIND_MAP: Record<AuthoringKind, ProtoConfigurationKind> = {
  'post-template': ProtoConfigurationKind.POST_TEMPLATE,
  'video-template': ProtoConfigurationKind.VIDEO_TEMPLATE,
  'post-guideline': ProtoConfigurationKind.POST_GUIDELINE,
  'video-guideline': ProtoConfigurationKind.VIDEO_GUIDELINE,
  'writing-voice': ProtoConfigurationKind.WRITING_VOICE,
}
export const authoringKindToProto = (kind: AuthoringKind): ProtoConfigurationKind => KIND_MAP[kind]
export function authoringKindFromProto(value: ProtoConfigurationKind): AuthoringKind {
  const found = (Object.keys(KIND_MAP) as AuthoringKind[]).find((kind) => KIND_MAP[kind] === value)
  if (!found) throw new Error('Unsupported configuration kind')
  return found
}
export const authoringModeToProto = (mode: AuthoringMode) =>
  mode === 'recommend' ? ProtoAuthoringMode.RECOMMEND : ProtoAuthoringMode.REFINE

export function mapAuthoringSession(wire: WireSession, scope: AuthoringScope): AuthoringSession {
  const kind = authoringKindFromProto(wire.kind)
  const targetId = scope.targetId ?? ''
  // The captured source target stays immutable for operation/receipt replay.
  // A new non-voice setting also becomes recoverable under its published id.
  const publishedTarget =
    kind !== 'writing-voice' &&
    wire.targetId === '' &&
    targetId !== '' &&
    wire.saved?.id === targetId
  if (
    !wire.id ||
    kind !== scope.kind ||
    (wire.targetId !== targetId && !publishedTarget) ||
    !Number.isInteger(wire.revision) ||
    wire.revision < 0 ||
    !AUTHORING_PHASES.includes(wire.phase as AuthoringSession['phase'])
  )
    throw new Error('Authoring session identity unavailable')
  const count = wire.candidateCount || AUTHORING_SUGGESTION_COUNT
  if (!AUTHORING_CANDIDATE_COUNTS.includes(count as AuthoringCandidateCount))
    throw new Error('Authoring candidate count unavailable')
  const candidates = wire.candidates.map(
    ({ id, name, description, body, titleArea, revision }) => ({
      id,
      revision,
      name,
      description,
      body,
      titleArea,
    }),
  )
  if (
    candidates.length !== 0 &&
    (candidates.length !== count ||
      new Set(candidates.map((candidate) => candidate.id)).size !== count ||
      candidates.some(
        (candidate) => !candidate.id || !candidate.name.trim() || !candidate.body.trim(),
      ))
  )
    throw new Error('Authoring suggestions unavailable')
  if (
    wire.selected &&
    (!wire.selected.id ||
      (!wire.selected.name.trim() && kind !== 'post-guideline' && kind !== 'video-guideline') ||
      (!wire.selected.body.trim() &&
        !(kind === 'writing-voice' && !!wire.targetId && wire.phase !== 'saved')))
  )
    throw new Error('Authoring draft unavailable')
  if (wire.saved && (!wire.saved.id || authoringKindFromProto(wire.saved.kind) !== kind))
    throw new Error('Authoring save unavailable')
  if (wire.phase === 'saved' && !wire.saved) throw new Error('Authoring save unconfirmed')
  return {
    id: wire.id,
    kind,
    workingSource: wire.workingSource
      ? mapArtifact(wire.workingSource)
      : wire.selected
        ? mapArtifact(wire.selected)
        : undefined,
    savedBaseline: wire.savedBaseline ? mapArtifact(wire.savedBaseline) : undefined,
    draftState: authoringDraftStateFromProto(wire.draftState),
    hasUnpublishedChanges: wire.hasUnpublishedChanges,
    savedAvailable:
      wire.savedAvailable ||
      (!wire.savedBaseline &&
        wire.failureReason !== 'AUTHORING_SAVE_CONFLICT' &&
        (!!wire.targetId || !!wire.saved)),
    candidateCount: count as AuthoringCandidateCount,
    revision: wire.revision,
    phase: wire.phase as AuthoringSession['phase'],
    targetId,
    targetVersion: wire.targetVersion,
    candidates,
    selected: wire.selected
      ? {
          id: wire.selected.id,
          name: wire.selected.name,
          description: wire.selected.description,
          body: wire.selected.body,
          titleArea: wire.selected.titleArea,
        }
      : undefined,
    turns: wire.turns.map(({ id, request, reply, jobId, status }) => ({
      id,
      request,
      reply,
      jobId,
      status,
    })),
    activeJobId: wire.activeJobId,
    saved: wire.saved ? { kind, id: wire.saved.id, name: wire.saved.name } : undefined,
    failureReason: wire.failureReason,
    pendingRequest: wire.pendingRequest,
  }
}
export function mapAuthoringEstimate(wire: { free: boolean; credits?: bigint }): AuthoringEstimate {
  if (wire.free) return { free: true, credits: 0 }
  if (
    wire.credits === undefined ||
    wire.credits < 0n ||
    wire.credits > BigInt(Number.MAX_SAFE_INTEGER)
  )
    throw new Error('Authoring estimate unavailable')
  return { free: false, credits: Number(wire.credits) }
}

function mapArtifact(a: NonNullable<WireSession['selected']>) {
  return {
    id: a.id,
    revision: a.revision,
    name: a.name,
    description: a.description,
    body: a.body,
    titleArea: a.titleArea,
  }
}
export function authoringDraftStateFromProto(value: ProtoAuthoringDraftState): AuthoringDraftState {
  switch (value) {
    case ProtoAuthoringDraftState.UNSPECIFIED:
      return 'valid'
    case ProtoAuthoringDraftState.VALID:
      return 'valid'
    case ProtoAuthoringDraftState.INCOMPLETE:
      return 'incomplete'
    case ProtoAuthoringDraftState.INVALID:
      return 'invalid'
  }
  throw new Error('Authoring draft state unavailable')
}
export type WireSummary = Awaited<ReturnType<Client['listAuthoringSummaries']>>['summaries'][number]
export function mapAuthoringSummary(wire: WireSummary, kind: AuthoringKind): AuthoringSummary {
  if (
    !wire.sessionId ||
    authoringKindFromProto(wire.kind) !== kind ||
    !Number.isInteger(wire.revision) ||
    wire.revision < 0 ||
    !wire.updatedAt
  )
    throw new Error('Authoring summary unavailable')
  if (
    wire.lastPublication &&
    (!wire.lastPublication.id || authoringKindFromProto(wire.lastPublication.kind) !== kind)
  )
    throw new Error('Authoring publication unavailable')
  return {
    sessionId: wire.sessionId,
    kind,
    targetId: wire.targetId,
    displayName: wire.displayName,
    revision: wire.revision,
    savedAvailable: wire.savedAvailable,
    hasUnpublishedChanges: wire.hasUnpublishedChanges,
    activeJobId: wire.activeJobId,
    publicationPending: wire.publicationPending,
    targetConflict: wire.targetConflict,
    draftState: authoringDraftStateFromProto(wire.draftState),
    updatedAt: wire.updatedAt,
    lastPublication: wire.lastPublication
      ? { kind, id: wire.lastPublication.id, name: wire.lastPublication.name }
      : undefined,
  }
}
