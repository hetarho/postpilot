import type { RequestInspectionView } from '@/shared/api'

interface InspectionOwner {
  ownerId: string
  /** Host identity for current local material that is not represented by a server revision. */
  contextKey?: string
}

export type InspectionAuthoringKind =
  'post-template' | 'video-template' | 'post-guideline' | 'video-guideline' | 'writing-voice'

export type RequestInspectionTarget = InspectionOwner &
  (
    | {
        kind: 'post'
        postSlug: string
        sourceRevision: string
        resultRevision: string
        planRevision?: string
      }
    | {
        kind: 'authoring'
        sessionId: string
        authoringKind: InspectionAuthoringKind
        revision: number
        mode: 'recommend' | 'refine'
        prompt?: string
        model?: { providerId: string; modelId: string }
        candidateCount?: number
        candidateId?: string
        operationId?: string
      }
    | {
        kind: 'test'
        testId: string
        candidateId: string
        revision: number
        blind: boolean
        payloadExpiresAt?: string
      }
  )

export interface RequestInspectionSelection {
  stage: string
  status: 'current' | 'prepared' | 'captured'
}

export interface RequestInspectionRead {
  inspection: RequestInspectionView
  /** All actual captured calls, in server order. Missing history is one unavailable view. */
  inspections: RequestInspectionView[]
}

/** Includes every prospective input as well as the resource and its publication fences. */
export function requestInspectionTargetKey(target: RequestInspectionTarget | null): string {
  if (!target) return ''
  const scope = [target.ownerId, target.kind, target.contextKey ?? '']
  switch (target.kind) {
    case 'post':
      return JSON.stringify([
        ...scope,
        target.postSlug,
        target.sourceRevision,
        target.resultRevision,
        target.planRevision ?? '',
      ])
    case 'authoring':
      return JSON.stringify([
        ...scope,
        target.sessionId,
        target.authoringKind,
        target.revision,
        target.mode,
        target.prompt ?? '',
        target.model?.providerId ?? '',
        target.model?.modelId ?? '',
        target.candidateCount ?? 0,
        target.candidateId ?? '',
        target.operationId ?? '',
      ])
    case 'test':
      return JSON.stringify([
        ...scope,
        target.testId,
        target.candidateId,
        target.revision,
        target.blind,
        target.payloadExpiresAt ?? '',
      ])
  }
}
