import { createClient } from '@connectrpc/connect'
import {
  ConfigurationAuthoringService,
  ProtoConfigurationKind,
  ProtoAuthoringMode,
} from '@/shared/api'
import {
  AUTHORING_PHASES,
  AUTHORING_SUGGESTION_COUNT,
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
  if (
    !wire.id ||
    kind !== scope.kind ||
    wire.targetId !== (scope.targetId ?? '') ||
    !Number.isInteger(wire.revision) ||
    wire.revision < 0 ||
    !AUTHORING_PHASES.includes(wire.phase as AuthoringSession['phase'])
  )
    throw new Error('Authoring session identity unavailable')
  const candidates = wire.candidates.map(({ id, name, description, body, titleArea }) => ({
    id,
    name,
    description,
    body,
    titleArea,
  }))
  if (
    candidates.length !== 0 &&
    (candidates.length !== AUTHORING_SUGGESTION_COUNT ||
      new Set(candidates.map((candidate) => candidate.id)).size !== AUTHORING_SUGGESTION_COUNT ||
      candidates.some(
        (candidate) => !candidate.id || !candidate.name.trim() || !candidate.body.trim(),
      ))
  )
    throw new Error('Authoring suggestions unavailable')
  if (
    wire.selected &&
    (!wire.selected.id || !wire.selected.name.trim() || !wire.selected.body.trim())
  )
    throw new Error('Authoring draft unavailable')
  if (wire.saved && (!wire.saved.id || authoringKindFromProto(wire.saved.kind) !== kind))
    throw new Error('Authoring save unavailable')
  if (wire.phase === 'saved' && !wire.saved) throw new Error('Authoring save unconfirmed')
  return {
    id: wire.id,
    kind,
    revision: wire.revision,
    phase: wire.phase as AuthoringSession['phase'],
    targetId: wire.targetId,
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
