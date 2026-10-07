import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import {
  ConfigurationAuthoringService as Service,
  normalizeAppFailure,
  ProtoAuthoringMode,
  ProtoConfigurationKind,
} from '@/shared/api'
import { TEST_COUNTS, type TestSettingKind } from '../model/types'
import type {
  CandidatePreparationClient,
  CandidatePreparationScope,
  PreparedCandidates,
} from '../model/preparation'
import { requireRevision, requireText, safeCount, WritingTestValidationError } from './mappers'

const kinds: Record<TestSettingKind, ProtoConfigurationKind> = {
  'writing-voice': ProtoConfigurationKind.WRITING_VOICE,
  'post-template': ProtoConfigurationKind.POST_TEMPLATE,
  'post-guideline': ProtoConfigurationKind.POST_GUIDELINE,
}
function check(scope: CandidatePreparationScope) {
  if (!TEST_COUNTS.includes(scope.count) || !kinds[scope.kind])
    throw new WritingTestValidationError('preparation format')
}
export function createCandidatePreparationClient(transport: Transport): CandidatePreparationClient {
  const client = createClient(Service, transport)
  const map = (
    wire: Awaited<ReturnType<typeof client.getAuthoringSession>>['session'],
    scope: CandidatePreparationScope,
    id?: string,
  ): PreparedCandidates => {
    if (
      !wire ||
      !wire.id ||
      (id && wire.id !== id) ||
      wire.kind !== kinds[scope.kind] ||
      wire.targetId ||
      wire.saved
    )
      throw new WritingTestValidationError('prepared session identity')
    requireRevision(wire.revision)
    if (
      !['choosing', 'editing', 'generating', 'refining', 'failed', 'cancelled'].includes(wire.phase)
    )
      throw new WritingTestValidationError('prepared phase')
    if (
      wire.candidates.length &&
      (wire.candidateCount !== scope.count ||
        wire.candidates.length !== scope.count ||
        new Set(wire.candidates.map((candidate) => candidate.id)).size !== scope.count)
    )
      throw new WritingTestValidationError('prepared candidate count')
    const candidates = wire.candidates.map((candidate) => {
      requireText(candidate.id, 'prepared candidate')
      requireText(candidate.name, 'prepared name')
      requireText(candidate.body, 'prepared body')
      requireRevision(candidate.revision)
      // T622 exposes artifact revisions separately from the mutable session. No fabricated fallback.
      if (candidate.revision === 0)
        throw new WritingTestValidationError('prepared candidate revision')
      return {
        id: candidate.id,
        name: candidate.name,
        description: candidate.description,
        body: candidate.body,
        titleArea: candidate.titleArea,
        revision: candidate.revision,
        source: {
          type: 'authoring' as const,
          authoring: {
            sessionId: wire.id,
            candidateId: candidate.id,
            revision: candidate.revision,
          },
        },
      }
    })
    const status = wire.activeJobId
      ? 'running'
      : wire.phase === 'failed'
        ? 'failed'
        : wire.phase === 'cancelled'
          ? 'cancelled'
          : candidates.length === scope.count
            ? 'ready'
            : 'idle'
    return {
      ...scope,
      sessionId: wire.id,
      revision: wire.revision,
      status,
      activeJobId: wire.activeJobId,
      candidates,
      failure: wire.failureReason
        ? normalizeAppFailure({ reason: wire.failureReason, params: {} })
        : undefined,
    }
  }
  return {
    estimate: async (input, signal) => {
      check(input)
      requireText(input.writeModel.providerId, 'preparation provider')
      requireText(input.writeModel.modelId, 'preparation model')
      const quote = await client.estimateAuthoringOperation(
        {
          kind: kinds[input.kind],
          mode: ProtoAuthoringMode.RECOMMEND,
          candidateCount: input.count,
          sessionId: input.sessionId ?? '',
          writeModel: input.writeModel,
        },
        { signal },
      )
      const credits =
        quote.free && quote.credits === undefined
          ? 0
          : safeCount(
              quote.credits ??
                (() => {
                  throw new WritingTestValidationError('preparation credits')
                })(),
              'preparation credits',
            )
      if (quote.free && credits !== 0)
        throw new WritingTestValidationError('preparation free credits')
      return { free: quote.free, credits }
    },
    create: async (input, signal) => {
      check(input)
      requireText(input.requestKey, 'preparation operation')
      return map(
        (
          await client.createAuthoringSession(
            { kind: kinds[input.kind], targetId: '', requestId: input.requestKey },
            { signal },
          )
        ).session,
        input,
      )
    },
    start: async (input, signal) => {
      check(input)
      requireText(input.sessionId, 'preparation session')
      requireRevision(input.expectedRevision)
      requireText(input.requestKey, 'preparation operation')
      requireText(input.prompt, 'preparation request')
      if (Array.from(input.prompt).length > 2000)
        throw new WritingTestValidationError('preparation request length')
      requireText(input.writeModel.providerId, 'preparation provider')
      requireText(input.writeModel.modelId, 'preparation model')
      const wire = await client.startAuthoringOperation(
        {
          sessionId: input.sessionId,
          expectedRevision: input.expectedRevision,
          requestId: input.requestKey,
          mode: ProtoAuthoringMode.RECOMMEND,
          prompt: input.prompt,
          writeModel: input.writeModel,
          candidateCount: input.count,
        },
        { signal },
      )
      if (!wire.jobId) throw new WritingTestValidationError('preparation job')
      const result = map(wire.session, input, input.sessionId)
      if (result.activeJobId && result.activeJobId !== wire.jobId)
        throw new WritingTestValidationError('preparation job identity')
      return result
    },
    get: async (input, signal) => {
      check(input)
      requireText(input.sessionId, 'preparation session')
      return map(
        (await client.getAuthoringSession({ sessionId: input.sessionId }, { signal })).session,
        input,
        input.sessionId,
      )
    },
    cancel: async (input, signal) => {
      check(input)
      requireText(input.sessionId, 'preparation session')
      requireText(input.jobId, 'preparation job')
      return map(
        (
          await client.cancelAuthoringOperation(
            { sessionId: input.sessionId, jobId: input.jobId },
            { signal },
          )
        ).session,
        input,
        input.sessionId,
      )
    },
  }
}
export function useCandidatePreparationClient(): CandidatePreparationClient {
  const transport = useTransport()
  return useMemo(() => createCandidatePreparationClient(transport), [transport])
}
