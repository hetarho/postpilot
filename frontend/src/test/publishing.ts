import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  type AppFailureReason,
  CancelPublishResponseSchema,
  CreateAgentPairingResponseSchema,
  GetPublishJobResponseSchema,
  ListRetryablePublishJobsResponseSchema,
  ListPublishingAgentsResponseSchema,
  PublishJobSchema,
  PublishStatus,
  PublishingAgentSchema,
  PublishingService,
  RevokePublishingAgentResponseSchema,
  RetryPublishResponseSchema,
  StartPublishResponseSchema,
  UpdatePublishingAgentResponseSchema,
  type ProtoPublishJob,
  type ProtoPublishingAgent,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

type FakePublishingMutation = 'pair' | 'configure' | 'revoke' | 'retry' | 'cancel'

export interface FakePublishingMutationFailure {
  reason: AppFailureReason
  code?: Code
  params?: Record<string, string>
}

export interface FakePublishingOptions {
  calls?: string[]
  agents?: ProtoPublishingAgent[]
  jobs?: ProtoPublishJob[]
  startRequests?: Array<{
    expectedContentRevision: bigint
    agentId: string
    categoryId: string
    visibility: number
  }>
  mutationFailures?: Partial<Record<FakePublishingMutation, FakePublishingMutationFailure>>
}

export function registerPublishingService(
  router: ConnectRouter,
  options: FakePublishingOptions = {},
) {
  const agents = [...(options.agents ?? [])]
  const jobs = [...(options.jobs ?? [])]
  const failMutation = (mutation: FakePublishingMutation) => {
    const failure = options.mutationFailures?.[mutation]
    if (failure) {
      throw connectAppError(failure.reason, failure.code ?? Code.InvalidArgument, failure.params)
    }
  }
  router.rpc(PublishingService.method.listPublishingAgents, () => {
    options.calls?.push('ListPublishingAgents')
    return create(ListPublishingAgentsResponseSchema, { agents })
  })
  router.rpc(PublishingService.method.getPublishJob, (request) => {
    options.calls?.push('GetPublishJob')
    const job = jobs.find(
      (candidate) => candidate.id === request.jobId || candidate.postSlug === request.postSlug,
    )
    if (!job) throw connectAppError('PUBLISH_NOT_FOUND', Code.NotFound)
    return create(GetPublishJobResponseSchema, { job })
  })
  router.rpc(PublishingService.method.listRetryablePublishJobs, () => {
    options.calls?.push('ListRetryablePublishJobs')
    return create(ListRetryablePublishJobsResponseSchema, {
      jobs: jobs.filter((job) => job.status === PublishStatus.NEEDS_ATTENTION),
    })
  })
  router.rpc(PublishingService.method.retryPublish, (request) => {
    options.calls?.push('RetryPublish')
    failMutation('retry')
    const index = jobs.findIndex((job) => job.id === request.jobId)
    if (index < 0) throw connectAppError('PUBLISH_NOT_FOUND', Code.NotFound)
    const job = create(PublishJobSchema, { ...jobs[index], status: 1, stage: 1 })
    jobs[index] = job
    return create(RetryPublishResponseSchema, { job })
  })
  router.rpc(PublishingService.method.startPublish, (request) => {
    options.calls?.push('StartPublish')
    options.startRequests?.push({
      expectedContentRevision: request.expectedContentRevision,
      agentId: request.agentId,
      categoryId: request.categoryId,
      visibility: request.visibility,
    })
    const job = create(PublishJobSchema, {
      id: 'publish-job-new',
      postSlug: request.postSlug,
      agentId: request.agentId,
      status: 1,
      stage: 1,
      contentRevision: request.expectedContentRevision,
    })
    jobs.push(job)
    return create(StartPublishResponseSchema, { job })
  })
  router.rpc(PublishingService.method.cancelPublish, (request) => {
    options.calls?.push('CancelPublish')
    failMutation('cancel')
    const index = jobs.findIndex((job) => job.id === request.jobId)
    if (index < 0) throw connectAppError('PUBLISH_NOT_FOUND', Code.NotFound)
    const job = create(PublishJobSchema, { ...jobs[index], status: 7 })
    jobs[index] = job
    return create(CancelPublishResponseSchema, { job })
  })
  router.rpc(PublishingService.method.createAgentPairing, () => {
    options.calls?.push('CreateAgentPairing')
    failMutation('pair')
    return create(CreateAgentPairingResponseSchema, {
      deviceCode: 'ABCD-1234-EF56',
      expiresAt: '2026-08-30T12:10:00Z',
    })
  })
  router.rpc(PublishingService.method.updatePublishingAgent, (request) => {
    options.calls?.push('UpdatePublishingAgent')
    failMutation('configure')
    const existing = agents.find((agent) => agent.id === request.agentId)
    if (!existing) throw connectAppError('PUBLISH_AGENT_UNAVAILABLE', Code.NotFound)
    const agent = create(PublishingAgentSchema, {
      ...existing,
      label: request.label,
      defaultCategoryId: request.defaultCategoryId,
      defaultVisibility: request.defaultVisibility,
    })
    return create(UpdatePublishingAgentResponseSchema, { agent })
  })
  router.rpc(PublishingService.method.revokePublishingAgent, () => {
    options.calls?.push('RevokePublishingAgent')
    failMutation('revoke')
    return create(RevokePublishingAgentResponseSchema, {})
  })
}
