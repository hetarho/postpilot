import type { ModelRef } from '@/entities/model-catalog'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import {
  ExperimentOrigin,
  ExperimentSource,
  LeaderboardScope,
  LeaderboardWindow,
  ModelExperimentService,
  StartExperimentResponseSchema,
  ExperimentStatus,
  Stage,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeWriteExperimentStart {
  postSlug: string
  /** Which verdict form the started comparison will offer. */
  origin: ExperimentOrigin
  observeModel?: ModelRef
  modelA?: ModelRef
  modelB?: ModelRef
  targetLength?: number
  /** The re-observation picker's answer, presence preserved as in FakeGenerationStart. */
  reobserveFiles?: string[]
}

export interface FakeExperimentsOptions {
  observeStarts?: Array<{ postSlug: string; modelA?: ModelRef; modelB?: ModelRef }>
  history?: Array<{
    id: string
    stage: Stage
    postSlug?: string
    voiceId?: string
    source?: ExperimentSource
    voicePromptText?: string
  }>
  reads?: Array<{
    kind: 'history' | 'leaderboard'
    stage: Stage
    /** The history's source filter; UNSPECIFIED for every source and for a board. */
    source?: ExperimentSource
    window?: LeaderboardWindow
    scope?: LeaderboardScope
  }>
  listFails?: boolean
  leaderboardFails?: boolean
  detailFails?: boolean
  readGate?: Promise<void>
  starts?: FakeWriteExperimentStart[]
  calls?: string[]
  jobId?: string
  experimentId?: string
  startError?: string
  /** 말투 반영 비교 starts the fake received (MODEL-67). */
  reflectionStarts?: Array<{
    voiceId: string
    promptKey: string
    modelA?: ModelRef
    modelB?: ModelRef
  }>
}

export function registerExperimentService(
  router: ConnectRouter,
  options: FakeExperimentsOptions = {},
) {
  // Like the server: the lab compares observe and write alone (MODEL-30), so a history or a
  // board naming analyze is refused rather than answered empty.
  const refuseAnalyze = (stage: Stage) => {
    if (stage === Stage.ANALYZE)
      throw connectAppError('EXPERIMENT_STAGE_INVALID', Code.InvalidArgument)
  }
  router.rpc(ModelExperimentService.method.listExperiments, async (request) => {
    options.reads?.push({ kind: 'history', stage: request.stage, source: request.source })
    refuseAnalyze(request.stage)
    if (options.readGate) await options.readGate
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    // Like the server: an unstated source is every one, and a row without one is post-sourced.
    const sourceOf = (item: { source?: ExperimentSource }) => item.source ?? ExperimentSource.POST
    return {
      experiments: (options.history ?? [])
        .filter((item) => request.stage === Stage.UNSPECIFIED || item.stage === request.stage)
        .filter(
          (item) =>
            request.source === ExperimentSource.UNSPECIFIED || sourceOf(item) === request.source,
        )
        .map((item) => ({ ...item, status: ExperimentStatus.DECIDED })),
    }
  })
  router.rpc(ModelExperimentService.method.getLeaderboard, async (request) => {
    options.reads?.push({
      kind: 'leaderboard',
      stage: request.stage,
      window: request.window,
      scope: request.scope,
    })
    refuseAnalyze(request.stage)
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
  router.rpc(ModelExperimentService.method.startVoiceReflectionExperiment, (request) => {
    options.calls?.push('StartVoiceReflectionExperiment')
    if (options.startError) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const ref = (value?: { providerId: string; modelId: string }) =>
      value ? { providerId: value.providerId, modelId: value.modelId } : undefined
    options.reflectionStarts?.push({
      voiceId: request.voiceId,
      promptKey: request.promptKey,
      modelA: ref(request.modelA),
      modelB: ref(request.modelB),
    })
    return create(StartExperimentResponseSchema, {
      jobId: options.jobId ?? 'experiment-job',
      experimentId: options.experimentId ?? 'experiment-1',
    })
  })
  router.rpc(ModelExperimentService.method.startWriteExperiment, (request) => {
    options.calls?.push('StartWriteExperiment')
    if (options.startError) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    options.starts?.push({
      postSlug: request.postSlug,
      origin: request.origin,
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
}
