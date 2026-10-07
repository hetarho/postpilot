import { createClient, type Transport } from '@connectrpc/connect'
import {
  InspectionStatus,
  ProtoAuthoringMode,
  ProtoConfigurationKind,
  WritingInspectionService,
  requestInspectionFromProto,
  type RequestInspection,
  type RequestInspectionView,
} from '@/shared/api'
import type {
  InspectionAuthoringKind,
  RequestInspectionRead,
  RequestInspectionSelection,
  RequestInspectionTarget,
} from '../model/types'

const statuses = {
  current: InspectionStatus.CURRENT,
  prepared: InspectionStatus.PREPARED,
  captured: InspectionStatus.CAPTURED,
} as const
const kinds: Record<InspectionAuthoringKind, ProtoConfigurationKind> = {
  'post-template': ProtoConfigurationKind.POST_TEMPLATE,
  'video-template': ProtoConfigurationKind.VIDEO_TEMPLATE,
  'post-guideline': ProtoConfigurationKind.POST_GUIDELINE,
  'video-guideline': ProtoConfigurationKind.VIDEO_GUIDELINE,
  'writing-voice': ProtoConfigurationKind.WRITING_VOICE,
}

export function unavailableInspection(stage: string, reason?: string): RequestInspectionRead {
  const inspection: RequestInspectionView = {
    version: 1,
    status: 'unavailable',
    stage,
    mode: '',
    fragments: [],
    selectedRuleIds: [],
    ...(reason ? { unavailableReason: reason } : {}),
  }
  return { inspection, inspections: [inspection] }
}

function readProjection(
  selection: RequestInspectionSelection,
  primary?: RequestInspection,
  calls: RequestInspection[] = [],
): RequestInspectionRead {
  const inspections = (calls.length ? calls : [primary]).map((call) => {
    const view = requestInspectionFromProto(call)
    // A preview returned to a history read is never evidence of an issued request.
    return view.status === selection.status || view.status === 'unavailable'
      ? view
      : unavailableInspection(selection.stage).inspection
  })
  return { inspection: inspections[inspections.length - 1]!, inspections }
}

/** The only exposed methods are authenticated reads. No generation, estimate or settings client. */
export function createRequestInspectionReader(transport: Transport) {
  const client = createClient(WritingInspectionService, transport)
  return async (
    target: RequestInspectionTarget,
    selection: RequestInspectionSelection,
    signal?: AbortSignal,
  ): Promise<RequestInspectionRead> => {
    const input = { stage: selection.stage, status: statuses[selection.status] }
    switch (target.kind) {
      case 'post': {
        const response = await client.getPostRequestInspection(
          { ...input, postSlug: target.postSlug },
          { signal },
        )
        return readProjection(selection, response.inspection, response.inspections)
      }
      case 'authoring': {
        const response = await client.getAuthoringRequestInspection(
          {
            ...input,
            // session_id is canonical; the old draft_id alias is intentionally omitted.
            sessionId: target.sessionId,
            kind: kinds[target.authoringKind],
            revision: target.revision,
            mode:
              target.mode === 'recommend'
                ? ProtoAuthoringMode.RECOMMEND
                : ProtoAuthoringMode.REFINE,
            prompt: target.prompt ?? '',
            model: target.model,
            candidateCount: target.candidateCount ?? 0,
          },
          { signal },
        )
        return readProjection(selection, response.inspection)
      }
      case 'test': {
        const response = await client.getWritingTestRequestInspection(
          { ...input, testId: target.testId, candidateId: target.candidateId },
          { signal },
        )
        // The server denies identity-bearing projections for the whole blind lifecycle.
        // Discard an unexpected projection wholesale as an additional client guard.
        const projection = readProjection(selection, response.inspection, response.inspections)
        if (target.blind && projection.inspections.some((view) => view.status !== 'unavailable'))
          return unavailableInspection(selection.stage, 'blind_test_identity_hidden_until_reveal')
        return projection
      }
    }
  }
}
