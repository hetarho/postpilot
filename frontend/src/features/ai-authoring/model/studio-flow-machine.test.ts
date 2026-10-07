import { createActor } from 'xstate'
import { describe, expect, it, vi } from 'vitest'
import { authoringScopeKey, type AuthoringSession } from '@/entities/ai-authoring'
import type { AuthoringState } from './authoring-machine'
import { studioFlowMachine, studioFlowView, type StudioFlowEvent } from './studio-flow-machine'

const scope = { ownerId: 'alice', kind: 'post-guideline' as const }
const scopeKey = authoringScopeKey(scope)
const candidates = Array.from({ length: 8 }, (_, n) => ({
  id: 'candidate-' + n,
  name: '제안 ' + n,
  description: '명확한 글쓰기 방향',
  body: '실제 경험을 쉽게 설명해 주세요.',
  titleArea: '',
}))
function session(patch: Partial<AuthoringSession> = {}): AuthoringSession {
  return {
    id: 'session',
    kind: scope.kind,
    targetId: '',
    targetVersion: '',
    revision: 1,
    phase: patch.selected ? 'editing' : 'choosing',
    candidates,
    turns: [],
    activeJobId: '',
    failureReason: '',
    pendingRequest: '',
    ...patch,
  }
}
function operation(patch: Partial<AuthoringState> = {}): AuthoringState {
  return { scopeKey, phase: 'idle', operation: 0, text: '', ...patch }
}
function actor(initial = operation()) {
  const select = vi.fn(),
    recommend = vi.fn(),
    refine = vi.fn(),
    publish = vi.fn(),
    cancel = vi.fn(),
    load = vi.fn()
  const value = createActor(
    studioFlowMachine.provide({
      actions: {
        requestRecommend: recommend,
        requestRefine: refine,
        selectCandidate: select,
        publish,
        cancel,
        loadExisting: load,
      },
    }),
    { input: { scope, operation: initial, ai: 'ready' } },
  ).start()
  const send = (event: Omit<StudioFlowEvent, 'scopeKey'>) =>
    value.send({ ...event, scopeKey } as StudioFlowEvent)
  const observe = (next: AuthoringState) =>
    value.send({ type: 'OBSERVE', scopeKey, operation: next, ai: 'ready' })
  return {
    value,
    send,
    observe,
    select,
    recommend,
    refine,
    publish,
    cancel,
    load,
    view: () => studioFlowView(value.getSnapshot()),
  }
}

describe('scoped studio presentation actor', () => {
  it('allows an empty optional purpose and admits a repeated recommendation only once', () => {
    const h = actor()
    expect(h.view()).toBe('purpose')
    h.send({ type: 'RECOMMEND' })
    h.send({ type: 'RECOMMEND' })
    expect(h.view()).toBe('working')
    expect(h.recommend).toHaveBeenCalledOnce()
  })
  it('shows a local length error after an attempt and preserves purpose across Back', () => {
    const h = actor()
    h.send({ type: 'PURPOSE_CHANGED', text: '가'.repeat(2001) } as StudioFlowEvent)
    h.send({ type: 'RECOMMEND' })
    expect(h.value.getSnapshot().context.problem).toBe('length')
    expect(h.recommend).not.toHaveBeenCalled()
    h.send({ type: 'PURPOSE_CHANGED', text: '경험을 편하게 설명하고 싶어요' } as StudioFlowEvent)
    h.send({ type: 'RECOMMEND' })
    h.observe(
      operation({
        phase: 'active',
        operation: 1,
        session: session({ phase: 'generating', activeJobId: 'job', candidates: [] }),
      }),
    )
    h.observe(operation({ phase: 'choosing', operation: 1, session: session({ revision: 2 }) }))
    expect(h.view()).toBe('choices')
    h.send({ type: 'BACK' })
    expect(h.view()).toBe('purpose')
    expect(h.value.getSnapshot().context.purpose).toBe('경험을 편하게 설명하고 싶어요')
    expect(h.recommend).toHaveBeenCalledOnce()
  })
  it('rejects an unknown choice, reviews selection and changes views without selecting again', () => {
    const chosen = candidates[0]!
    const h = actor(operation({ phase: 'choosing', session: session() }))
    h.send({ type: 'CHOOSE', candidateId: 'unknown' } as StudioFlowEvent)
    expect(h.select).not.toHaveBeenCalled()
    h.send({ type: 'CHOOSE', candidateId: chosen.id } as StudioFlowEvent)
    h.send({ type: 'CHOOSE', candidateId: candidates[1]!.id } as StudioFlowEvent)
    expect(h.select).toHaveBeenCalledOnce()
    h.observe(
      operation({
        phase: 'editing',
        operation: 1,
        session: session({ revision: 2, phase: 'editing', selected: chosen }),
      }),
    )
    expect(h.view()).toBe('review')
    h.send({ type: 'CHANGE_SELECTION' })
    expect(h.view()).toBe('choices')
    h.send({ type: 'CHOOSE', candidateId: chosen.id } as StudioFlowEvent)
    expect(h.view()).toBe('review')
    expect(h.select).toHaveBeenCalledOnce()
    expect(h.recommend).not.toHaveBeenCalled()
  })
  it('publishes a reviewed valid selection without any chat request', () => {
    const h = actor(
      operation({
        phase: 'editing',
        session: session({ phase: 'editing', selected: candidates[0] }),
      }),
    )
    h.send({ type: 'PUBLISH' })
    expect(h.publish).not.toHaveBeenCalled()
    h.send({ type: 'OPEN_PUBLICATION' })
    expect(h.view()).toBe('publication')
    h.send({ type: 'BACK' })
    expect(h.view()).toBe('review')
    h.send({ type: 'OPEN_PUBLICATION' })
    h.send({ type: 'PUBLISH' })
    h.send({ type: 'PUBLISH' })
    expect(h.publish).toHaveBeenCalledOnce()
    expect(h.refine).not.toHaveBeenCalled()
  })
  it('requires an explicit chat opening and explains a blank message after an attempt', () => {
    const h = actor(
      operation({
        phase: 'editing',
        session: session({ phase: 'editing', selected: candidates[0] }),
      }),
    )
    h.send({ type: 'REFINE' })
    expect(h.refine).not.toHaveBeenCalled()
    h.send({ type: 'OPEN_REFINEMENT' })
    expect(h.view()).toBe('refining')
    h.send({ type: 'REFINE' })
    expect(h.value.getSnapshot().context.problem).toBe('required')
    h.send({ type: 'BACK' })
    expect(h.view()).toBe('review')
    expect(h.publish).not.toHaveBeenCalled()
  })
  it('requires a draft example before publication of a personal seed', () => {
    const h = actor(
      operation({
        phase: 'editing',
        session: session({ selected: { ...candidates[0]!, body: '' } }),
      }),
    )
    h.send({ type: 'OPEN_PUBLICATION' })
    expect(h.view()).toBe('review')
    expect(h.publish).not.toHaveBeenCalled()
    h.send({ type: 'OPEN_REFINEMENT' })
    expect(h.view()).toBe('refining')
  })
  it('only confirms cancellation explicitly and preserves the modal across polling', () => {
    const active = operation({
      phase: 'active',
      operation: 1,
      session: session({ phase: 'refining', activeJobId: 'job', selected: candidates[0] }),
    })
    const h = actor(active)
    h.send({ type: 'ASK_CANCEL' })
    expect(h.value.getSnapshot().matches({ working: 'confirmingCancellation' })).toBe(true)
    h.observe({ ...active })
    expect(h.value.getSnapshot().matches({ working: 'confirmingCancellation' })).toBe(true)
    h.send({ type: 'DISMISS_CANCEL' })
    expect(h.cancel).not.toHaveBeenCalled()
    h.send({ type: 'ASK_CANCEL' })
    h.send({ type: 'CONFIRM_CANCEL' })
    h.send({ type: 'CONFIRM_CANCEL' })
    expect(h.cancel).toHaveBeenCalledOnce()
  })
  it('closes obsolete cancellation confirmation when the durable job finishes', () => {
    const h = actor(
      operation({
        phase: 'active',
        session: session({ phase: 'generating', activeJobId: 'job', candidates: [] }),
      }),
    )
    h.send({ type: 'ASK_CANCEL' })
    h.observe(operation({ phase: 'choosing', session: session({ revision: 2 }) }))
    expect(h.view()).toBe('choices')
    h.send({ type: 'CONFIRM_CANCEL' })
    expect(h.cancel).not.toHaveBeenCalled()
  })
  it('rejects another owner, an older operation and a stale session revision', () => {
    const current = operation({
      phase: 'editing',
      operation: 3,
      session: session({ revision: 5, selected: candidates[0] }),
    })
    const h = actor(current)
    h.value.send({ type: 'OPEN_PUBLICATION', scopeKey: 'bob' })
    h.observe({ ...current, operation: 2, phase: 'saved' })
    h.observe({ ...current, phase: 'saved', session: session({ revision: 4 }) })
    expect(h.view()).toBe('review')
    expect(h.value.getSnapshot().context.operation).toBe(current)
  })
  it('recovers interrupted publication with an explicit enabled save action', () => {
    const h = actor(
      operation({
        phase: 'active',
        session: session({ phase: 'saving', selected: candidates[0], activeJobId: '' }),
      }),
    )
    expect(h.view()).toBe('publication')
    h.send({ type: 'PUBLISH' })
    expect(h.publish).toHaveBeenCalledOnce()
    expect(h.refine).not.toHaveBeenCalled()
  })
  it('returns a dismissed estimate to the same comparison without any new work', () => {
    const initial = operation({ phase: 'choosing', session: session() })
    const h = actor(initial)
    h.send({ type: 'RECOMMEND' })
    h.observe({ ...initial, phase: 'confirming', operation: 1 })
    h.observe({ ...initial, operation: 1 })
    expect(h.view()).toBe('choices')
    expect(h.recommend).toHaveBeenCalledOnce()
  })
})
