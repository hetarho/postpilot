import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { CandidatePreparationClient, PreparedCandidates } from '@/entities/writing-test'
import { useCandidatePreparation } from './useCandidatePreparation'
import type { PreparationDraft } from './candidate-preparation-machine'

afterEach(() => sessionStorage.clear())
const draft: PreparationDraft = {
  kind: 'writing-voice',
  count: 16,
  prompt: 'Prepare exactly sixteen varied styles.',
  writeModel: { providerId: 'p', modelId: 'writer' },
}
function session(status: 'idle' | 'running' = 'idle'): PreparedCandidates {
  return {
    kind: 'writing-voice',
    count: 16,
    sessionId: 'session',
    revision: status === 'idle' ? 1 : 2,
    status,
    activeJobId: status === 'running' ? 'job' : '',
    candidates: [],
  }
}
function ready(): PreparedCandidates {
  return {
    ...session('running'),
    status: 'ready',
    activeJobId: '',
    candidates: Array.from({ length: 16 }, (_, index) => ({
      id: `candidate-${index}`,
      name: `Style ${index}`,
      description: '',
      body: `Full style ${index}`,
      titleArea: '',
      revision: 1,
      source: {
        type: 'authoring' as const,
        authoring: { sessionId: 'session', candidateId: `candidate-${index}`, revision: 1 },
      },
    })),
  }
}
function client(): CandidatePreparationClient {
  return {
    estimate: vi.fn(async () => ({ free: true, credits: 0 })),
    create: vi.fn(async () => session()),
    start: vi.fn(async () => session('running')),
    get: vi.fn(async () => session()),
    cancel: vi.fn(),
  }
}
it('count changes retain the typed direction and Back/resize/rerender never start preparation', async () => {
  const api = client()
  const hook = renderHook(
    ({ count }: { count: 2 | 16 }) =>
      useCandidatePreparation({
        ownerId: 'alice',
        seedKey: 'count',
        client: api,
        draft: { ...draft, count, prompt: '' },
      }),
    { initialProps: { count: 2 } },
  )
  const original = hook.result.current.actorRef
  act(() =>
    hook.result.current.send({
      type: 'EDIT',
      draft: { ...draft, count: 2, prompt: 'Retain this direction' },
    }),
  )
  hook.rerender({ count: 16 })
  expect(hook.result.current.context.draft.count).toBe(16)
  expect(hook.result.current.context.draft.prompt).toBe('Retain this direction')
  act(() => hook.result.current.send({ type: 'ESTIMATE' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('quoted'))
  act(() => hook.result.current.send({ type: 'BACK' }))
  window.dispatchEvent(new Event('resize'))
  expect(hook.result.current.actorRef).toBe(original)
  expect(api.create).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  hook.unmount()
})
it('lost create persists its kind/keys and remount never automatically creates or starts work', async () => {
  const api = client()
  vi.mocked(api.create)
    .mockRejectedValueOnce(new Error('create response lost'))
    .mockResolvedValueOnce(session())
  const input = { ownerId: 'alice', seedKey: 'lost-create', client: api, draft }
  const hook = renderHook(() => useCandidatePreparation(input))
  act(() => hook.result.current.send({ type: 'ESTIMATE' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('quoted'))
  act(() => hook.result.current.send({ type: 'CONFIRM' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('uncertain'))
  hook.unmount()
  const restored = renderHook(() => useCandidatePreparation(input))
  expect(restored.result.current.phase).toBe('uncertain')
  expect(api.create).toHaveBeenCalledTimes(1)
  expect(api.start).not.toHaveBeenCalled()
  act(() => restored.result.current.send({ type: 'RETRY' }))
  await waitFor(() => expect(restored.result.current.phase).toBe('created'))
  expect(api.start).not.toHaveBeenCalled()
  expect(vi.mocked(api.create).mock.calls[1]![0]).toEqual(vi.mocked(api.create).mock.calls[0]![0])
  restored.unmount()
})
it('owner changes fence stale callbacks even if the host omits its key', () => {
  const api = client()
  const hook = renderHook(
    ({ ownerId }) => useCandidatePreparation({ ownerId, seedKey: 'owner', client: api, draft }),
    { initialProps: { ownerId: 'alice' } },
  )
  const oldSend = hook.result.current.send
  hook.rerender({ ownerId: 'bob' })
  act(() => {
    oldSend({ type: 'ESTIMATE' })
    hook.result.current.send({ type: 'ESTIMATE' })
  })
  expect(hook.result.current.context.suspended).toBe(true)
  expect(api.estimate).not.toHaveBeenCalled()
  hook.unmount()
})
it('malformed or foreign recovery cannot restart paid work', () => {
  const api = client()
  const key = `postpilot:test-candidates:${JSON.stringify(['alice', 'invalid'])}`
  sessionStorage.setItem(
    key,
    JSON.stringify({
      scopeKey: JSON.stringify(['alice', 'invalid']),
      recovery: {
        draft: { kind: 'writing-voice', count: 16, prompt: null },
        pending: { kind: 'start', input: {} },
      },
    }),
  )
  const hook = renderHook(() =>
    useCandidatePreparation({ ownerId: 'alice', seedKey: 'invalid', client: api, draft }),
  )
  expect(hook.result.current.phase).toBe('idle')
  expect(hook.result.current.context.draft.prompt).toBe(draft.prompt)
  expect(api.start).not.toHaveBeenCalled()
  expect(api.create).not.toHaveBeenCalled()
  hook.unmount()
})
it('a readonly restored sixteen-candidate result survives an unchanged caller default count and issues no provider operation', async () => {
  const api = client()
  vi.mocked(api.get).mockResolvedValue(ready())
  const scopeKey = JSON.stringify(['alice', 'restored-count'])
  sessionStorage.setItem(
    `postpilot:test-candidates:${scopeKey}`,
    JSON.stringify({ scopeKey, recovery: { draft, session: session('running') } }),
  )
  const hook = renderHook(() =>
    useCandidatePreparation({
      ownerId: 'alice',
      seedKey: 'restored-count',
      client: api,
      draft: { ...draft, count: 2 },
    }),
  )
  await waitFor(() => expect(hook.result.current.phase).toBe('ready'))
  expect(hook.result.current.context.draft.count).toBe(16)
  expect(hook.result.current.context.session?.candidates).toHaveLength(16)
  expect(api.create).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  expect(api.estimate).not.toHaveBeenCalled()
  hook.unmount()
})

it('keeps an exact three-slot session and mapping after a four-entry plan is filled and across reload', async () => {
  const api = client()
  const partial: PreparationDraft = {
    ...draft,
    count: 3,
    testCount: 4,
    slotIndices: [1, 2, 3],
    retainedRefs: [],
  }
  const base = { ...session(), count: 3 as const }
  const completed = { ...ready(), count: 3 as const, candidates: ready().candidates.slice(0, 3) }
  vi.mocked(api.create).mockResolvedValue(base)
  vi.mocked(api.start).mockResolvedValue({
    ...base,
    status: 'running',
    revision: 2,
    activeJobId: 'job',
  })
  vi.mocked(api.get).mockResolvedValue(completed)
  const hook = renderHook(
    ({ currentDraft }) =>
      useCandidatePreparation({
        ownerId: 'alice',
        seedKey: 'mixed',
        client: api,
        draft: currentDraft,
      }),
    { initialProps: { currentDraft: partial } },
  )
  act(() => hook.result.current.send({ type: 'ESTIMATE' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('quoted'))
  act(() => hook.result.current.send({ type: 'CONFIRM' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('running'))
  act(() => hook.result.current.send({ type: 'REFRESH' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('ready'))
  const refs = completed.candidates.map((candidate) => candidate.source)
  const filled = { ...partial, count: 4 as const, slotIndices: undefined, retainedRefs: refs }
  hook.rerender({ currentDraft: filled })
  expect(hook.result.current.phase).toBe('ready')
  expect(hook.result.current.context.draft).toMatchObject({
    count: 3,
    testCount: 4,
    slotIndices: [1, 2, 3],
  })
  expect(hook.result.current.context.session?.count).toBe(3)
  expect(hook.result.current.context.artifacts).toHaveLength(3)
  hook.unmount()
  const restored = renderHook(() =>
    useCandidatePreparation({ ownerId: 'alice', seedKey: 'mixed', client: api, draft: filled }),
  )
  await waitFor(() => expect(restored.result.current.phase).toBe('ready'))
  expect(restored.result.current.context.artifacts).toHaveLength(3)
  expect(restored.result.current.context.draft.slotIndices).toEqual([1, 2, 3])
  expect(api.start).toHaveBeenCalledTimes(1)
  expect(api.create).toHaveBeenCalledTimes(1)
  restored.unmount()
})
it('does not expose private artifacts from a persisted owner substitution', () => {
  const api = client()
  const scopeKey = JSON.stringify(['bob', 'private'])
  const candidates = ready().candidates
  const inputDraft = { ...draft, retainedRefs: candidates.map((candidate) => candidate.source) }
  sessionStorage.setItem(
    `postpilot:test-candidates:${scopeKey}`,
    JSON.stringify({
      scopeKey,
      recovery: {
        ownerId: 'alice',
        draft: inputDraft,
        session: ready(),
        artifacts: candidates,
      },
    }),
  )
  const hook = renderHook(() =>
    useCandidatePreparation({ ownerId: 'bob', seedKey: 'private', client: api, draft: inputDraft }),
  )
  expect(hook.result.current.context.artifacts).toEqual([])
  expect(hook.result.current.context.session).toBeUndefined()
  expect(api.get).not.toHaveBeenCalled()
  hook.unmount()
})
