import { act, renderHook, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { expect, it, vi } from 'vitest'
import { PostContentSchema } from '@/shared/api'
import type { WritingTestClient, WritingTestPlan, WritingTest } from '@/entities/writing-test'
import { useWritingTestFlow } from './useWritingTestFlow'
import { writingTestScopeKey } from './writing-test-machine'

function plan(): WritingTestPlan {
  return {
    factor: 'model',
    modelStage: 'write',
    count: 2,
    entrants: [
      { type: 'model', model: { providerId: 'p', modelId: 'a' } },
      { type: 'model', model: { providerId: 'p', modelId: 'b' } },
    ],
    context: {
      sourcePostSlug: '',
      expectedInputRevision: 0n,
      expectedContentRevision: 0n,
      material: {
        text: 'Retained factual material',
        fictional: false,
        attachmentIds: [],
        templateAnswers: [],
      },
      voiceId: '',
      templateId: '',
      guidelineSlotId: '',
      targetLanguage: 'ko',
      targetLength: 1000,
      tagCount: 3,
      useMemory: false,
      qualityRules: [],
    },
  }
}
function test(): WritingTest {
  return {
    id: 'test',
    revision: 1,
    factor: 'model',
    modelStage: 'write',
    count: 2,
    status: 'review',
    sourcePostSlug: '',
    jobId: '',
    candidates: ['a', 'b'].map((id) => ({
      id,
      status: 'succeeded',
      displayLabel: id,
      output: create(PostContentSchema, { title: id }),
    })),
    matches: [
      {
        id: 'match',
        round: 1,
        index: 0,
        leftCandidateId: 'a',
        rightCandidateId: 'b',
        winnerCandidateId: '',
      },
    ],
    winnerCandidateId: '',
    publications: [],
    revealed: false,
    createdAt: '',
    updatedAt: '',
    contentExpiresAt: '2099-01-01T00:00:00Z',
    fictional: false,
    confirmedCredits: 0,
    reservedCredits: 0,
    targetLanguage: 'ko',
  }
}
function client(): WritingTestClient {
  return {
    estimate: vi.fn(async () => ({
      free: true,
      credits: 0,
      quoteKey: 'quote',
      expiresAt: '2099-01-01T00:00:00Z',
    })),
    estimateRetry: vi.fn(),
    start: vi.fn(async () => test()),
    get: vi.fn(async () => test()),
    list: vi.fn(),
    retry: vi.fn(),
    decide: vi.fn(),
    cancel: vi.fn(),
    saveWinner: vi.fn(),
    applyOutput: vi.fn(),
  }
}
function storage() {
  const records = new Map<string, string>()
  return {
    getItem: vi.fn((key: string) => records.get(key) ?? null),
    setItem: vi.fn((key: string, value: string) => {
      records.set(key, value)
    }),
  }
}
it('owner/seed actor lifetime survives rerender/resize, Back and remount retain draft without automatic provider work', async () => {
  const api = client()
  const persisted = storage()
  const options = {
    ownerId: 'alice',
    seedKey: 'draft',
    client: api,
    initialDraft: plan(),
    recoveryStorage: persisted,
  }
  const hook = renderHook(() => useWritingTestFlow(options))
  const original = hook.result.current.actorRef
  act(() => {
    hook.result.current.sendPresentation({ type: 'NEXT' })
    hook.result.current.sendPresentation({ type: 'NEXT' })
    hook.result.current.sendPresentation({ type: 'BACK' })
  })
  expect(hook.result.current.view).toBe('candidates')
  hook.rerender()
  window.dispatchEvent(new Event('resize'))
  expect(hook.result.current.actorRef).toBe(original)
  expect(api.start).not.toHaveBeenCalled()
  expect(api.estimate).not.toHaveBeenCalled()
  expect(api.get).not.toHaveBeenCalled()
  hook.unmount()
  const resumed = renderHook(() => useWritingTestFlow({ ...options, initialDraft: undefined }))
  expect(resumed.result.current.context.draft?.context.material.text).toBe(
    'Retained factual material',
  )
  expect(resumed.result.current.context.draft?.context.expectedInputRevision).toBe(0n)
  expect(resumed.result.current.view).toBe('candidates')
  expect(api.start).not.toHaveBeenCalled()
  resumed.unmount()
})
it('persists uncertain start and permits only an explicit same-key replay after remount', async () => {
  const api = client()
  vi.mocked(api.start)
    .mockRejectedValueOnce(new Error('response lost'))
    .mockResolvedValueOnce(test())
  const persisted = storage()
  const options = {
    ownerId: 'alice',
    seedKey: 'draft',
    client: api,
    initialDraft: plan(),
    recoveryStorage: persisted,
  }
  const hook = renderHook(() => useWritingTestFlow(options))
  act(() => hook.result.current.send({ type: 'ESTIMATE' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('quoted'))
  act(() => hook.result.current.send({ type: 'CONFIRM' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('uncertain'))
  hook.unmount()
  const resumed = renderHook(() => useWritingTestFlow(options))
  expect(resumed.result.current.phase).toBe('uncertain')
  expect(api.start).toHaveBeenCalledTimes(1)
  expect(api.get).not.toHaveBeenCalled()
  act(() => resumed.result.current.send({ type: 'RETRY_OPERATION' }))
  await waitFor(() => expect(resumed.result.current.phase).toBe('match'))
  expect(vi.mocked(api.start).mock.calls[1]![0]).toEqual(vi.mocked(api.start).mock.calls[0]![0])
  resumed.unmount()
})
it('resume reads the server and restores reading positions; closing never cancels or votes', async () => {
  const api = client()
  const persisted = storage()
  const options = {
    ownerId: 'alice',
    seedKey: 'test',
    testId: 'test',
    client: api,
    recoveryStorage: persisted,
  }
  const hook = renderHook(() => useWritingTestFlow(options))
  await waitFor(() => expect(hook.result.current.phase).toBe('match'))
  act(() => {
    hook.result.current.sendPresentation({ type: 'READING', candidateId: 'a', position: 700 })
    hook.result.current.sendPresentation({ type: 'SHOW_CANDIDATE', candidateId: 'b' })
  })
  hook.unmount()
  const resumed = renderHook(() => useWritingTestFlow(options))
  await waitFor(() => expect(resumed.result.current.phase).toBe('match'))
  expect(resumed.result.current.presentation.reading.a).toBe(700)
  expect(resumed.result.current.presentation.visibleCandidateId).toBe('b')
  expect(api.get).toHaveBeenCalledTimes(2)
  expect(api.decide).not.toHaveBeenCalled()
  expect(api.cancel).not.toHaveBeenCalled()
  expect(api.start).not.toHaveBeenCalled()
  resumed.unmount()
})
it('fences a changed owner even if a host omits its required remount key', () => {
  const api = client()
  const hook = renderHook(
    ({ ownerId }) =>
      useWritingTestFlow({
        ownerId,
        seedKey: 'draft',
        client: api,
        initialDraft: plan(),
        recoveryStorage: null,
      }),
    { initialProps: { ownerId: 'alice' } },
  )
  const oldSend = hook.result.current.send
  hook.rerender({ ownerId: 'bob' })
  act(() => {
    oldSend({ type: 'ESTIMATE' })
    hook.result.current.send({ type: 'ESTIMATE' })
  })
  expect(api.estimate).not.toHaveBeenCalled()
  expect(hook.result.current.context.suspended).toBe(true)
  hook.unmount()
})
it('Back from an estimate unlocks the retained material for correction without starting generation', async () => {
  const api = client()
  const hook = renderHook(() =>
    useWritingTestFlow({
      ownerId: 'alice',
      seedKey: 'draft',
      client: api,
      initialDraft: plan(),
      recoveryStorage: null,
    }),
  )
  act(() => hook.result.current.send({ type: 'ESTIMATE' }))
  await waitFor(() => expect(hook.result.current.phase).toBe('quoted'))
  act(() => hook.result.current.sendPresentation({ type: 'BACK' }))
  expect(hook.result.current.phase).toBe('editing')
  const changed = plan()
  changed.context.material.text = 'Corrected material'
  act(() => hook.result.current.send({ type: 'DRAFT', draft: changed }))
  expect(hook.result.current.context.draft?.context.material.text).toBe('Corrected material')
  expect(api.start).not.toHaveBeenCalled()
  expect(api.estimate).toHaveBeenCalledTimes(1)
  hook.unmount()
})
it('ignores malformed browser recovery instead of crashing or resuming an unvalidated paid command', () => {
  const persisted = storage()
  const key = `postpilot:writing-test:${writingTestScopeKey('alice', 'draft')}`
  persisted.setItem(
    key,
    JSON.stringify({
      scopeKey: writingTestScopeKey('alice', 'draft'),
      operation: {
        draft: { count: 16, entrants: [null], context: { material: null } },
        command: { kind: 'start', input: { requestKey: 'hostile' } },
      },
      presentation: { reading: { a: 'wrong' } },
    }),
  )
  const api = client()
  const hook = renderHook(() =>
    useWritingTestFlow({
      ownerId: 'alice',
      seedKey: 'draft',
      client: api,
      initialDraft: plan(),
      recoveryStorage: persisted,
    }),
  )
  expect(hook.result.current.context.draft).toEqual(plan())
  expect(hook.result.current.phase).toBe('editing')
  expect(api.start).not.toHaveBeenCalled()
  hook.unmount()
})
