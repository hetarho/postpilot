import type { ModelRef } from '@/entities/model-catalog'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import {
  ModelExperimentService,
  StartExperimentResponseSchema,
  ExperimentStatus,
  Stage,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeWriteExperimentStart {
  postSlug: string
  observeModel?: ModelRef
  modelA?: ModelRef
  modelB?: ModelRef
  targetLength?: number
  /** The re-observation picker's answer, presence preserved as in FakeGenerationStart. */
  reobserveFiles?: string[]
}

export interface FakeAnalyzeExperimentStart {
  voiceId: string
  modelA?: ModelRef
  modelB?: ModelRef
}

export interface FakeExperimentsOptions {
  observeStarts?: Array<{ postSlug: string; modelA?: ModelRef; modelB?: ModelRef }>
  history?: Array<{ id: string; stage: Stage; postSlug?: string; voiceId?: string }>
  reads?: Array<{ kind: 'history' | 'leaderboard'; stage: Stage }>
  listFails?: boolean
  leaderboardFails?: boolean
  detailFails?: boolean
  readGate?: Promise<void>
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
  router.rpc(ModelExperimentService.method.listExperiments, async (request) => {
    options.reads?.push({ kind: 'history', stage: request.stage })
    if (options.readGate) await options.readGate
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return {
      experiments: (options.history ?? [])
        .filter((item) => request.stage === Stage.UNSPECIFIED || item.stage === request.stage)
        .map((item) => ({ ...item, status: ExperimentStatus.DECIDED })),
    }
  })
  router.rpc(ModelExperimentService.method.getLeaderboard, async (request) => {
    options.reads?.push({ kind: 'leaderboard', stage: request.stage })
    if (options.readGate) await options.readGate
    if (options.leaderboardFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return { entries: [] }
  })
  router.rpc(ModelExperimentService.method.getExperiment, async (request) => {
    if (options.readGate) await options.readGate
    if (options.detailFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const item = options.history?.find((item) => item.id === request.id)
    return { experiment: item ? { ...item, status: ExperimentStatus.DECIDED } : undefined }
  })
  router.rpc(ModelExperimentService.method.startObserveExperiment, (request) => {
    options.calls?.push('StartObserveExperiment')
    if (options.startError) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    options.observeStarts?.push({
      postSlug: request.postSlug,
      modelA: request.modelA
        ? { providerId: request.modelA.providerId, modelId: request.modelA.modelId }
        : undefined,
      modelB: request.modelB
        ? { providerId: request.modelB.providerId, modelId: request.modelB.modelId }
        : undefined,
    })
    return {
      jobId: options.jobId ?? 'experiment-job',
      experimentId: options.experimentId ?? 'experiment-1',
    }
  })
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
      reobserveFiles: request.reobserve ? request.reobserve.files : undefined,
    })
    return create(StartExperimentResponseSchema, {
      jobId: options.jobId ?? 'experiment-job',
      experimentId: options.experimentId ?? 'experiment-1',
    })
  })
  router.rpc(ModelExperimentService.method.startAnalyzeExperiment, (request) => {
    options.calls?.push('StartAnalyzeExperiment')
    if (options.startError) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    // The server never guesses a voice (spec/legacy/policy/model-experiments.md).
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
