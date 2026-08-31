import {
  appFailureFromProto,
  contentLanguageFromProto,
  type ProtoGenerationJob,
} from '@/shared/api'
import type { GenerationJob, ModelRef } from '../model/types'

function toModelRef(ref: ProtoGenerationJob['observeModel']): ModelRef | undefined {
  if (!ref) return undefined
  return { providerId: ref.providerId, modelId: ref.modelId }
}

export function toGenerationJob(job: ProtoGenerationJob): GenerationJob {
  return {
    id: job.id,
    kind: job.kind,
    status: job.status,
    stage: job.stage,
    progressDone: job.progressDone,
    progressTotal: job.progressTotal,
    failure: job.failure || job.status === 'failed' ? appFailureFromProto(job.failure) : undefined,
    postSlug: job.postSlug,
    observeModel: toModelRef(job.observeModel),
    writeModel: toModelRef(job.writeModel),
    createdAt: job.createdAt,
    updatedAt: job.updatedAt,
    targetLanguage: contentLanguageFromProto(job.targetLanguage),
  }
}
