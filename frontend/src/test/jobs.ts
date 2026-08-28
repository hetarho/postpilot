import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  GenerationJobSchema,
  GenerationService,
  GetGenerationResponseSchema,
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
}

export function createFakeJobsTransport(options: FakeJobsOptions = {}) {
  return createRouterTransport((router) => registerGenerationService(router, options))
}
