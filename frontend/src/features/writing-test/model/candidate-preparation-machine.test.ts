import { createActor, waitFor, type ActorRefFrom } from 'xstate'
import { expect, it, vi } from 'vitest'
import type {
  CandidatePreparationClient,
  PreparedCandidates,
  TestCount,
} from '@/entities/writing-test'
import {
  candidatePreparationMachine,
  preparationRecovery,
  type PreparationDraft,
  type PreparationRecovery,
} from './candidate-preparation-machine'

const scopeKey = JSON.stringify(['alice', 'prepare'])
const draft: PreparationDraft = {
  kind: 'writing-voice',
  count: 16,
  prompt: 'Sixteen different writing styles, using fictional examples.',
  writeModel: { providerId: 'p', modelId: 'writer' },
}
function session(
  count: TestCount = 16,
  status: PreparedCandidates['status'] = 'idle',
  changes: Partial<PreparedCandidates> = {},
): PreparedCandidates {
  return {
    kind: 'writing-voice',
    count,
    sessionId: 'session',
    revision: status === 'idle' ? 1 : 8,
    status,
    activeJobId: status === 'running' ? 'job' : '',
    candidates:
      status === 'ready'
        ? Array.from({ length: count }, (_, i) => ({
            id: `candidate-${i}`,
            name: `Style ${i + 1}`,
            description: 'Synthetic fictional style',
            body: `Complete style instruction ${i}`,
            titleArea: '',
            revision: i + 1,
            source: {
              type: 'authoring' as const,
              authoring: { sessionId: 'session', candidateId: `candidate-${i}`, revision: i + 1 },
            },
          }))
        : [],
    ...changes,
  }
}
function client(count: TestCount = 16): CandidatePreparationClient {
  return {
    estimate: vi.fn(async () => ({ free: false, credits: 64 })),
    create: vi.fn(async () => session(count)),
    start: vi.fn(async () => session(count, 'running')),
    get: vi.fn(async () => session(count, 'ready')),
    cancel: vi.fn(async () => session(count, 'cancelled')),
  }
}
function actor(
  api: CandidatePreparationClient,
  options: { count?: TestCount; recovery?: PreparationRecovery } = {},
): ActorRefFrom<typeof candidatePreparationMachine> {
  let key = 0
  return createActor(candidatePreparationMachine, {
    input: {
      ownerId: 'alice',
      seedKey: 'prepare',
      client: api,
      draft: { ...draft, count: options.count ?? 16 },
      requestKey: () => `request-${++key}`,
      recovery: options.recovery,
    },
  }).start()
}
function recovery(ref: ReturnType<typeof actor>): PreparationRecovery {
  const context = ref.getSnapshot().context
  return {
    draft: context.draft,
    command: context.command,
    pending: context.pending,
    session: context.session,
    estimate: context.estimate,
  }
}
async function confirm(ref: ReturnType<typeof actor>) {
  ref.send({ type: 'ESTIMATE', scopeKey })
  ref.send({ type: 'ESTIMATE', scopeKey })
  await waitFor(ref, (s) => s.matches('quoted'))
  ref.send({ type: 'CONFIRM', scopeKey })
  ref.send({ type: 'CONFIRM', scopeKey })
}
it.each([2, 4, 8, 16] as const)(
  'prepares exactly %i unsaved candidates using separate explicit estimate/confirm and preserves each artifact version',
  async (count) => {
    const api = client(count)
    const ref = actor(api, { count })
    expect(api.create).not.toHaveBeenCalled()
    expect(api.start).not.toHaveBeenCalled()
    await confirm(ref)
    await waitFor(ref, (s) => s.matches('running'))
    expect(api.estimate).toHaveBeenCalledTimes(1)
    expect(api.create).toHaveBeenCalledTimes(1)
    expect(api.start).toHaveBeenCalledExactlyOnceWith(
      {
        kind: 'writing-voice',
        count,
        sessionId: 'session',
        expectedRevision: 1,
        requestKey: 'request-2',
        prompt: draft.prompt,
        writeModel: draft.writeModel,
      },
      expect.any(AbortSignal),
    )
    ref.send({ type: 'REFRESH', scopeKey })
    await waitFor(ref, (s) => s.matches('ready'))
    expect(ref.getSnapshot().context.session?.candidates).toHaveLength(count)
    expect(
      ref
        .getSnapshot()
        .context.session?.candidates.map((candidate) => candidate.source.authoring.revision),
    ).toEqual(Array.from({ length: count }, (_, i) => i + 1))
    expect(ref.getSnapshot().context.command).toBeUndefined()
    ref.stop()
  },
)
it('Back and editing preserve the direction and never create/start candidates', async () => {
  const api = client()
  const ref = actor(api)
  ref.send({ type: 'ESTIMATE', scopeKey })
  await waitFor(ref, (s) => s.matches('quoted'))
  ref.send({ type: 'BACK', scopeKey })
  expect(ref.getSnapshot().context.draft.prompt).toBe(draft.prompt)
  expect(api.create).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  ref.send({ type: 'EDIT', scopeKey, draft: { ...draft, prompt: '' } })
  ref.send({ type: 'ESTIMATE', scopeKey })
  expect(api.estimate).toHaveBeenCalledTimes(1)
  ref.stop()
})
it('lost create restores its original key, never auto-starts, and needs explicit confirmation after its session is recovered', async () => {
  const api = client()
  vi.mocked(api.create)
    .mockRejectedValueOnce(new Error('lost create'))
    .mockResolvedValueOnce(session())
  const ref = actor(api)
  await confirm(ref)
  await waitFor(ref, (s) => s.matches('uncertain'))
  const saved = recovery(ref)
  ref.stop()
  const resumed = actor(api, { recovery: saved })
  expect(resumed.getSnapshot().matches('uncertain')).toBe(true)
  resumed.send({ type: 'EDIT', scopeKey, draft: { ...draft, count: 2 } })
  resumed.send({ type: 'ESTIMATE', scopeKey })
  expect(api.estimate).toHaveBeenCalledTimes(1)
  resumed.send({ type: 'RETRY', scopeKey })
  resumed.send({ type: 'RETRY', scopeKey })
  await waitFor(resumed, (s) => s.matches('created'))
  expect(api.start).not.toHaveBeenCalled()
  expect(vi.mocked(api.create).mock.calls[1]![0]).toEqual(vi.mocked(api.create).mock.calls[0]![0])
  resumed.send({ type: 'CONFIRM', scopeKey })
  await waitFor(resumed, (s) => s.matches('running'))
  expect(api.start).toHaveBeenCalledTimes(1)
  resumed.stop()
})
it('lost start stays fenced when readonly recovery returns the old idle session and retries the same start payload/key only explicitly', async () => {
  const api = client()
  vi.mocked(api.start)
    .mockRejectedValueOnce(new Error('lost start'))
    .mockResolvedValueOnce(session(16, 'running'))
  vi.mocked(api.get).mockResolvedValue(session())
  const ref = actor(api)
  await confirm(ref)
  await waitFor(ref, (s) => s.matches('uncertain'))
  const saved = recovery(ref)
  ref.stop()
  const resumed = actor(api, { recovery: saved })
  await waitFor(resumed, (s) => s.matches('uncertain'))
  expect(api.start).toHaveBeenCalledTimes(1)
  resumed.send({ type: 'EDIT', scopeKey, draft: { ...draft, prompt: 'new generation' } })
  resumed.send({ type: 'ESTIMATE', scopeKey })
  expect(resumed.getSnapshot().context.draft.prompt).toBe(draft.prompt)
  resumed.send({ type: 'RETRY', scopeKey })
  await waitFor(resumed, (s) => s.matches('running'))
  expect(vi.mocked(api.start).mock.calls[1]![0]).toEqual(vi.mocked(api.start).mock.calls[0]![0])
  expect(api.create).toHaveBeenCalledTimes(1)
  resumed.stop()
})
it('a received lost start is recovered from the durable job without starting again', async () => {
  const api = client()
  vi.mocked(api.start).mockRejectedValue(new Error('lost start'))
  const ref = actor(api)
  await confirm(ref)
  await waitFor(ref, (s) => s.matches('uncertain'))
  ref.send({ type: 'REFRESH', scopeKey })
  await waitFor(ref, (s) => s.matches('ready'))
  expect(api.start).toHaveBeenCalledTimes(1)
  expect(api.create).toHaveBeenCalledTimes(1)
  ref.stop()
})
it('lost cancel retries that frozen session/job only and never falls through to start', async () => {
  const api = client()
  vi.mocked(api.cancel)
    .mockRejectedValueOnce(new Error('lost cancel'))
    .mockResolvedValueOnce(session(16, 'cancelled', { revision: 9 }))
  vi.mocked(api.get).mockResolvedValue(session(16, 'running'))
  const ref = actor(api)
  await confirm(ref)
  await waitFor(ref, (s) => s.matches('running'))
  ref.send({ type: 'CANCEL', scopeKey })
  ref.send({ type: 'CANCEL', scopeKey })
  await waitFor(ref, (s) => s.matches('uncertain'))
  const saved = recovery(ref)
  ref.stop()
  const resumed = actor(api, { recovery: saved })
  await waitFor(resumed, (s) => s.matches('uncertain'))
  expect(resumed.getSnapshot().context.pending?.kind).toBe('cancel')
  resumed.send({ type: 'RETRY', scopeKey })
  resumed.send({ type: 'RETRY', scopeKey })
  await waitFor(resumed, (s) => s.matches('failed'))
  expect(api.start).toHaveBeenCalledTimes(1)
  expect(api.cancel).toHaveBeenCalledTimes(2)
  expect(vi.mocked(api.cancel).mock.calls[1]![0]).toEqual(vi.mocked(api.cancel).mock.calls[0]![0])
  resumed.stop()
})
it('a failed readonly read retains successful candidate data and RETRY can only read until recovery settles', async () => {
  const api = client()
  const ready = session(16, 'ready')
  const ref = actor(api, { recovery: { draft, session: ready } })
  await waitFor(ref, (s) => s.matches('ready'))
  vi.mocked(api.get).mockRejectedValueOnce(new Error('read lost')).mockResolvedValueOnce(ready)
  ref.send({ type: 'REFRESH', scopeKey })
  await waitFor(ref, (s) => s.matches('uncertain'))
  ref.send({ type: 'EDIT', scopeKey, draft: { ...draft, count: 2 } })
  ref.send({ type: 'ESTIMATE', scopeKey })
  expect(ref.getSnapshot().context.session?.candidates).toHaveLength(16)
  expect(ref.getSnapshot().context.pending?.kind).toBe('read')
  ref.send({ type: 'RETRY', scopeKey })
  await waitFor(ref, (s) => s.matches('ready'))
  expect(api.create).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  expect(api.estimate).not.toHaveBeenCalled()
  ref.stop()
})
it('fences owner/kind/count/session/revision/operation changes and late responses after suspension', async () => {
  let resolve!: (value: PreparedCandidates) => void
  const api = client()
  vi.mocked(api.get).mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done
      }),
  )
  const ref = actor(api, { recovery: { draft, session: session(16, 'running') } })
  const operation = ref.getSnapshot().context.operation
  const base = session(16, 'ready')
  for (const [value, owner, op] of [
    [{ ...base, count: 8 }, scopeKey, operation],
    [{ ...base, kind: 'post-template' }, scopeKey, operation],
    [{ ...base, sessionId: 'foreign' }, scopeKey, operation],
    [{ ...base, revision: 2 }, scopeKey, operation],
    [base, 'foreign-owner', operation],
    [base, scopeKey, operation - 1],
  ] as const)
    ref.send({
      type: 'HYDRATE',
      scopeKey: owner,
      operation: op,
      session: value as PreparedCandidates,
    })
  expect(ref.getSnapshot().context.session?.status).toBe('running')
  ref.send({ type: 'SUSPEND', scopeKey: 'foreign' })
  expect(ref.getSnapshot().matches('reading')).toBe(true)
  ref.send({ type: 'SUSPEND', scopeKey })
  resolve(base)
  await Promise.resolve()
  await Promise.resolve()
  expect(ref.getSnapshot().matches('suspended')).toBe(true)
  expect(ref.getSnapshot().context.session?.status).toBe('running')
  expect(api.cancel).not.toHaveBeenCalled()
  ref.stop()
})
it('untrusted recovery rejects malformed drafts, zero artifact versions and mismatched frozen scopes', () => {
  expect(
    preparationRecovery({ draft: { prompt: null, count: 16, kind: 'writing-voice' } }),
  ).toBeUndefined()
  const invalid = session(16, 'ready')
  invalid.candidates[0]!.revision = 0
  expect(preparationRecovery({ draft, session: invalid })).toBeUndefined()
  expect(
    preparationRecovery({
      draft,
      pending: {
        kind: 'cancel',
        input: { kind: 'post-template', count: 16, sessionId: 'session', jobId: 'job' },
      },
    }),
  ).toBeUndefined()
  expect(
    preparationRecovery({
      draft,
      command: { ...draft, createKey: 'original-create', startKey: 'original-start' },
      pending: {
        kind: 'create',
        input: { kind: draft.kind, count: 16, requestKey: 'replacement-key' },
      },
    }),
  ).toBeUndefined()
})
it('a definite generation refusal permits correcting the unchanged direction/model without an implicit new paid call', async () => {
  const api = client()
  vi.mocked(api.start).mockRejectedValue({ reason: 'AUTHORING_FEATURE_UNAVAILABLE', params: {} })
  const ref = actor(api)
  await confirm(ref)
  await waitFor(ref, (s) => s.matches('failed'))
  expect(ref.getSnapshot().context.draft.prompt).toBe(draft.prompt)
  ref.send({
    type: 'EDIT',
    scopeKey,
    draft: { ...draft, writeModel: { providerId: 'p', modelId: 'eligible' } },
  })
  ref.send({ type: 'ESTIMATE', scopeKey })
  await waitFor(ref, (s) => s.matches('quoted'))
  expect(api.create).toHaveBeenCalledTimes(1)
  expect(api.start).toHaveBeenCalledTimes(1)
  ref.stop()
})
