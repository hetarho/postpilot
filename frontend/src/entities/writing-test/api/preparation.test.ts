import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { expect, it } from 'vitest'
import { ConfigurationAuthoringService as Service, ProtoConfigurationKind } from '@/shared/api'
import { createCandidatePreparationClient } from './preparation'

it('estimates exactly sixteen private settings separately, then preserves real artifact revision refs on read', async () => {
  const calls: string[] = []
  const session = {
    id: 'private-session',
    revision: 50,
    kind: ProtoConfigurationKind.WRITING_VOICE,
    phase: 'choosing',
    candidateCount: 16,
    candidates: Array.from({ length: 16 }, (_, index) => ({
      id: `draft-${index}`,
      name: `Style ${index + 1}`,
      body: 'Synthetic style specification',
      revision: index + 2,
    })),
  }
  const client = createCandidatePreparationClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.estimateAuthoringOperation, (request) => {
        calls.push('estimate')
        expect(request).toMatchObject({
          kind: ProtoConfigurationKind.WRITING_VOICE,
          candidateCount: 16,
          writeModel: { providerId: 'provider', modelId: 'preparer' },
        })
        return create(Service.method.estimateAuthoringOperation.output, { credits: 12n })
      })
      rpc(Service.method.getAuthoringSession, () => {
        calls.push('get')
        return create(Service.method.getAuthoringSession.output, { session })
      })
    }),
  )
  const scope = { kind: 'writing-voice' as const, count: 16 as const }
  expect(
    await client.estimate({
      ...scope,
      writeModel: { providerId: 'provider', modelId: 'preparer' },
    }),
  ).toEqual({ free: false, credits: 12 })
  expect(calls).toEqual(['estimate'])
  const result = await client.get({ ...scope, sessionId: 'private-session' })
  expect(result.candidates).toHaveLength(16)
  expect(result.candidates[0].source).toEqual({
    type: 'authoring',
    authoring: { sessionId: 'private-session', candidateId: 'draft-0', revision: 2 },
  })
  expect(result.revision).toBe(50)
  expect(calls).toEqual(['estimate', 'get'])
})
it('creates and starts only on explicit calls, passing requested count and the same idempotency key', async () => {
  const calls: unknown[] = []
  const session = {
    id: 'session',
    revision: 1,
    phase: 'choosing',
    kind: ProtoConfigurationKind.POST_TEMPLATE,
    candidateCount: 8,
  }
  const client = createCandidatePreparationClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.createAuthoringSession, (request) => {
        calls.push(request)
        return create(Service.method.createAuthoringSession.output, { session })
      })
      rpc(Service.method.startAuthoringOperation, (request) => {
        calls.push(request)
        return create(Service.method.startAuthoringOperation.output, {
          session: {
            ...session,
            revision: 2,
            phase: 'generating',
            candidateCount: 4,
            activeJobId: 'job',
          },
          jobId: 'job',
        })
      })
    }),
  )
  const scope = { kind: 'post-template' as const, count: 4 as const }
  const created = await client.create({ ...scope, requestKey: 'explicit-creation' })
  expect(created.status).toBe('idle')
  const started = await client.start({
    ...scope,
    sessionId: created.sessionId,
    expectedRevision: created.revision,
    requestKey: 'explicit-preparation',
    prompt: 'Four varied review templates',
    writeModel: { providerId: 'provider', modelId: 'preparer' },
  })
  expect(started.status).toBe('running')
  expect(calls).toEqual([
    expect.objectContaining({ requestId: 'explicit-creation', targetId: '' }),
    expect.objectContaining({
      requestId: 'explicit-preparation',
      expectedRevision: 1,
      candidateCount: 4,
    }),
  ])
})
it('refuses absent artifact revisions, inconsistent counts and scoped session substitution', async () => {
  let revision = 0
  let count = 2
  const client = createCandidatePreparationClient(
    createRouterTransport(({ rpc }) => {
      rpc(Service.method.getAuthoringSession, () =>
        create(Service.method.getAuthoringSession.output, {
          session: {
            id: 'session',
            revision: 33,
            kind: ProtoConfigurationKind.POST_TEMPLATE,
            phase: 'choosing',
            candidateCount: count,
            candidates: [
              { id: 'first', name: 'First', body: 'Draft', revision },
              { id: 'second', name: 'Second', body: 'Draft', revision },
            ],
          },
        }),
      )
    }),
  )
  const input = { kind: 'post-template' as const, count: 2 as const, sessionId: 'session' }
  await expect(client.get(input)).rejects.toThrow('prepared candidate revision')
  revision = 3
  count = 16
  await expect(client.get(input)).rejects.toThrow('prepared candidate count')
  count = 2
  await expect(client.get({ ...input, sessionId: 'foreign' })).rejects.toThrow(
    'prepared session identity',
  )
})
