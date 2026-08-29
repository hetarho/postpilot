import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  GenerationJobSchema,
  GenerationService,
  GetGenerationResponseSchema,
  StartGenerationResponseSchema,
  StartRevisionResponseSchema,
  type ProtoGenerationJob,
} from '@/shared/api'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeGenerationJobRow {
  id: string
  kind?: string
  status?: string
  stage?: string
  progressDone?: number
  progressTotal?: number
  error?: string
  postSlug?: string
}

export interface FakeJobsOptions {
  jobs?: FakeGenerationJobRow[]
  /** Optional next snapshots returned on successive reads of the same id. */
  sequence?: FakeGenerationJobRow[]
  calls?: string[]
  startJobId?: string
  startFails?: boolean
  starts?: FakeGenerationStart[]
  revisions?: FakeRevisionStart[]
}

export interface FakeGenerationStart {
  postSlug: string
  observeModel?: { providerId: string; modelId: string }
  writeModel?: { providerId: string; modelId: string }
  targetLength?: number
}

export interface FakeRevisionStart {
  postSlug: string
  instruction: string
  saveAsRule: boolean
  writeModel?: { providerId: string; modelId: string }
}

export function toFakeProto(row: FakeGenerationJobRow): ProtoGenerationJob {
  return create(GenerationJobSchema, {
    kind: 'generate',
    status: 'running',
    stage: 'observe',
    progressTotal: 1,
    ...row,
  })
}

export function registerGenerationService(router: ConnectRouter, options: FakeJobsOptions = {}) {
  const jobs = new Map((options.jobs ?? []).map((row) => [row.id, row]))
  let sequenceIndex = 0
  router.rpc(GenerationService.method.getGeneration, (req) => {
    options.calls?.push('GetGeneration')
    const sequenced = options.sequence?.[Math.min(sequenceIndex, options.sequence.length - 1)]
    sequenceIndex += 1
    const found = sequenced?.id === req.id ? sequenced : jobs.get(req.id)
    if (!found) throw new ConnectError('not found', Code.NotFound)
    return create(GetGenerationResponseSchema, { job: toFakeProto(found) })
  })
  router.rpc(GenerationService.method.startGeneration, (req) => {
    options.calls?.push('StartGeneration')
    if (options.startFails) throw new ConnectError('생성을 시작하지 못했어요.', Code.Unavailable)
    options.starts?.push({
      postSlug: req.postSlug,
      observeModel: req.observeModel
        ? { providerId: req.observeModel.providerId, modelId: req.observeModel.modelId }
        : undefined,
      writeModel: req.writeModel
        ? { providerId: req.writeModel.providerId, modelId: req.writeModel.modelId }
        : undefined,
      targetLength: req.targetLength,
    })
    return create(StartGenerationResponseSchema, { jobId: options.startJobId ?? 'job-started' })
  })
  router.rpc(GenerationService.method.startRevision, (req) => {
    options.calls?.push('StartRevision')
    if (options.startFails) throw new ConnectError('수정을 시작하지 못했어요.', Code.Unavailable)
    options.revisions?.push({
      postSlug: req.postSlug,
      instruction: req.instruction,
      saveAsRule: req.saveAsRule,
      writeModel: req.writeModel
        ? { providerId: req.writeModel.providerId, modelId: req.writeModel.modelId }
        : undefined,
    })
    return create(StartRevisionResponseSchema, { jobId: options.startJobId ?? 'job-started' })
  })
}

export function createFakeJobsTransport(options: FakeJobsOptions = {}) {
  return createRouterTransport((router) => registerGenerationService(router, options))
}
