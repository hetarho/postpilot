import { create } from '@bufbuild/protobuf'
import {
  FragmentAuthorship,
  InspectionRole,
  InspectionStatus,
  RequestInspectionSchema,
} from '@/shared/api'
import type { RequestInspectionTarget } from '../model/types'

export const postTarget = {
  ownerId: 'alice',
  kind: 'post',
  postSlug: 'owned-post',
  sourceRevision: 'inputs-1',
  resultRevision: 'result-1',
} satisfies RequestInspectionTarget

export function capturedInspection(callId = 'job:write:1') {
  return create(RequestInspectionSchema, {
    version: 1,
    status: InspectionStatus.CAPTURED,
    stage: 'post-writing',
    mode: 'direct',
    promptVersion: 'write-v1',
    schemaVersion: 'post-v1',
    callId,
    output: { name: 'post', version: 'post-v1', schema: '{}' },
    fragments: [
      {
        id: 'memo',
        role: InspectionRole.USER,
        authorship: FragmentAuthorship.ACCOUNT,
        materialRole: 'fact',
        text: 'owner private memo',
        sourceRefs: ['memo:1'],
      },
    ],
    conditions: { model: { providerId: 'fake', modelId: 'private-model' } },
    issuedAt: { seconds: 1n },
  })
}
