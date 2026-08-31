import type { ModelRef } from '@/entities/model-catalog'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { ModelExperimentService, StartExperimentResponseSchema } from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeWriteExperimentStart {
  postSlug: string
  observeModel?: ModelRef
  modelA?: ModelRef
  modelB?: ModelRef
  targetLength?: number
}

export interface FakeAnalyzeExperimentStart {
  voiceId: string
  modelA?: ModelRef
  modelB?: ModelRef
}

export interface FakeExperimentsOptions {
  starts?: FakeWriteExperimentStart[]
  analyzeStarts?: FakeAnalyzeExperimentStart[]
  calls?: string[]
  jobId?: string
  experimentId?: string
  startError?: string
}

export function registerExperimentService(
  router: ConnectRouter,
  options: FakeExperimentsOptions = {},
) {
  router.rpc(ModelExperimentService.method.startWriteExperiment, (request) => {
    options.calls?.push('StartWriteExperiment')
    if (options.startError) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    options.starts?.push({
      postSlug: request.postSlug,
      observeModel: request.observeModel
        ? { providerId: request.observeModel.providerId, modelId: request.observeModel.modelId }
        : undefined,
      modelA: request.modelA
        ? { providerId: request.modelA.providerId, modelId: request.modelA.modelId }
        : undefined,
      modelB: request.modelB
        ? { providerId: request.modelB.providerId, modelId: request.modelB.modelId }
        : undefined,
      targetLength: request.targetLength,
    })
    return create(StartExperimentResponseSchema, {
      jobId: options.jobId ?? 'experiment-job',
      experimentId: options.experimentId ?? 'experiment-1',
    })
  })
  router.rpc(ModelExperimentService.method.startAnalyzeExperiment, (request) => {
    options.calls?.push('StartAnalyzeExperiment')
    if (options.startError) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    // The server never guesses a voice (spec/policy/model-experiments.md).
    if (!request.voiceId) throw connectAppError('EXPERIMENT_VOICE_REQUIRED', Code.InvalidArgument)
    options.analyzeStarts?.push({
      voiceId: request.voiceId,
      modelA: request.modelA
        ? { providerId: request.modelA.providerId, modelId: request.modelA.modelId }
        : undefined,
      modelB: request.modelB
        ? { providerId: request.modelB.providerId, modelId: request.modelB.modelId }
        : undefined,
    })
    return create(StartExperimentResponseSchema, {
      jobId: options.jobId ?? 'experiment-job',
      experimentId: options.experimentId ?? 'experiment-1',
    })
  })
}
