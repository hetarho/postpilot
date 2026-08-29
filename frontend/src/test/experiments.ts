import type { ModelRef } from '@/entities/model-catalog'
import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { ModelExperimentService, StartExperimentResponseSchema } from '@/shared/api'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeWriteExperimentStart {
  postSlug: string
  observeModel?: ModelRef
  modelA?: ModelRef
  modelB?: ModelRef
  targetLength?: number
}

export interface FakeExperimentsOptions {
  starts?: FakeWriteExperimentStart[]
  calls?: string[]
  jobId?: string
  experimentId?: string
}

export function registerExperimentService(router: ConnectRouter, options: FakeExperimentsOptions = {}) {
  router.rpc(ModelExperimentService.method.startWriteExperiment, (request) => {
    options.calls?.push('StartWriteExperiment')
    options.starts?.push({
      postSlug: request.postSlug,
      observeModel: request.observeModel ? { providerId: request.observeModel.providerId, modelId: request.observeModel.modelId } : undefined,
      modelA: request.modelA ? { providerId: request.modelA.providerId, modelId: request.modelA.modelId } : undefined,
      modelB: request.modelB ? { providerId: request.modelB.providerId, modelId: request.modelB.modelId } : undefined,
      targetLength: request.targetLength,
    })
    return create(StartExperimentResponseSchema, {
      jobId: options.jobId ?? 'experiment-job',
      experimentId: options.experimentId ?? 'experiment-1',
    })
  })
}
