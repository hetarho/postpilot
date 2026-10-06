import { describe, expect, it } from 'vitest'
import type { AuthoringScope, AuthoringSession } from '@/entities/ai-authoring'
import {
  initialAuthoringState,
  authoringTransition as move,
  studioBusy,
  type FrozenAuthoringCommand,
  type AuthoringState,
} from './authoring-machine'

const scope: AuthoringScope = { ownerId: 'alice', kind: 'post-template' }
const initial = () => initialAuthoringState(scope)
const scopeKey = initial().scopeKey
const artifact = {
  id: 'a',
  name: '일상 이야기',
  description: '사진과 경험을 자연스럽게',
  body: '<write>경험</write>',
  titleArea: '',
}
function session(overrides: Partial<AuthoringSession> = {}): AuthoringSession {
  return {
    id: 'session',
    kind: scope.kind,
    revision: 1,
    phase: 'choosing',
    targetId: '',
    targetVersion: '',
    candidates: [],
    turns: [],
    activeJobId: '',
    failureReason: '',
    pendingRequest: '',
    ...overrides,
  }
}
const ready = (server: AuthoringSession | null = null) =>
  move(initial(), { type: 'hydrate', scopeKey, session: server })
const command = (
  state: AuthoringState,
  mode: 'recommend' | 'refine' = 'recommend',
): FrozenAuthoringCommand => ({
  mode,
  prompt: mode === 'refine' ? '더 짧게 해줘' : '',
  writeModel: { providerId: 'provider', modelId: 'recommended' },
  sessionId: state.session?.id ?? '',
  expectedRevision: state.session?.revision ?? 0,
  createRequestId: 'create-key',
  requestId: 'start-key',
})

describe('guarded authoring studio', () => {
  it('hydrates durable state without scheduling work and restores a failed request', () => {
    expect(ready().phase).toBe('idle')
    const recovered = ready(
      session({ revision: 4, phase: 'failed', selected: artifact, pendingRequest: '저장한 요청' }),
    )
    expect(recovered.text).toBe('저장한 요청')
    expect(recovered.command).toBeUndefined()
    expect(studioBusy(recovered)).toBe(false)
  })
  it('requires a frozen valid model and explicit quote confirmation before starting', () => {
    const base = ready()
    expect(move(base, { type: 'begin', scopeKey, phase: 'starting' })).toBe(base)
    const quoted = move(base, { type: 'quote', scopeKey, command: command(base) })
    expect(quoted.phase).toBe('quoting')
    expect(move(quoted, { type: 'quote', scopeKey, command: command(base) })).toBe(quoted)
    const confirmed = move(quoted, {
      type: 'quoted',
      scopeKey,
      operation: quoted.operation,
      estimate: { free: false, credits: 7 },
    })
    const pending = move(confirmed, { type: 'begin', scopeKey, phase: 'starting' })
    expect(pending.command?.confirmed).toBe(true)
    expect(pending.command?.writeModel).toEqual({ providerId: 'provider', modelId: 'recommended' })
    expect(move(pending, { type: 'begin', scopeKey, phase: 'starting' })).toBe(pending)
  })
  it('retains request keys, text and prior suggestions after an uncertain start response', () => {
    const base = ready(
      session({
        selected: artifact,
        candidates: Array.from({ length: 8 }, (_, i) => ({ ...artifact, id: String(i) })),
      }),
    )
    const quoted = move(base, { type: 'quote', scopeKey, command: command(base, 'refine') })
    const confirmed = move(quoted, {
      type: 'quoted',
      scopeKey,
      operation: quoted.operation,
      estimate: { free: true, credits: 0 },
    })
    const pending = move(confirmed, { type: 'begin', scopeKey, phase: 'starting' })
    const failed = move(pending, {
      type: 'failed',
      scopeKey,
      operation: pending.operation,
      failure: { reason: 'UNKNOWN_FAILURE', params: {} },
    })
    const retry = move(failed, { type: 'begin', scopeKey, phase: 'starting' })
    expect(retry.command?.requestId).toBe('start-key')
    expect(retry.command?.expectedRevision).toBe(1)
    expect(retry.session?.candidates).toEqual(base.session?.candidates)
    expect(retry.operation).toBe(pending.operation + 1)
  })
  it('does not turn a failed estimate into an unpriced request', () => {
    const base = ready()
    const pending = move(base, { type: 'quote', scopeKey, command: command(base) })
    const failed = move(pending, {
      type: 'failed',
      scopeKey,
      operation: pending.operation,
      failure: { reason: 'UNKNOWN_FAILURE', params: {} },
    })
    expect(move(failed, { type: 'begin', scopeKey, phase: 'starting' })).toBe(failed)
    expect(move(failed, { type: 'retry-quote', scopeKey }).phase).toBe('quoting')
  })
  it('fences other owners, kinds, targets, sessions, versions and obsolete operations', () => {
    const base = ready(session({ selected: artifact, revision: 8 }))
    expect(
      move(base, {
        type: 'hydrate',
        scopeKey: JSON.stringify(['bob', scope.kind, '']),
        session: session(),
      }),
    ).toBe(base)
    for (const bad of [
      session({ kind: 'writing-voice' }),
      session({ targetId: 'other' }),
      session({ id: 'other' }),
      session({ revision: 7 }),
    ])
      expect(move(base, { type: 'hydrate', scopeKey, session: bad })).toBe(base)
    const pending = move(base, { type: 'begin', scopeKey, phase: 'saving' })
    expect(
      move(pending, {
        type: 'response',
        scopeKey,
        operation: pending.operation - 1,
        session: session({ revision: 9 }),
      }),
    ).toBe(pending)
    expect(move(pending, { type: 'draft', scopeKey, text: 'new text' })).toBe(pending)
    expect(move(pending, { type: 'new-session', scopeKey })).toBe(pending)
  })
  it('enforces nonblank refinement, Unicode bounds and twenty completed exchanges', () => {
    const base = ready(session({ selected: artifact }))
    expect(
      move(base, { type: 'quote', scopeKey, command: { ...command(base, 'refine'), prompt: ' ' } }),
    ).toBe(base)
    expect(
      move(base, {
        type: 'quote',
        scopeKey,
        command: { ...command(base, 'refine'), prompt: '😀'.repeat(2001) },
      }),
    ).toBe(base)
    expect(
      move(base, {
        type: 'quote',
        scopeKey,
        command: { ...command(base, 'refine'), prompt: '😀'.repeat(2000) },
      }).phase,
    ).toBe('quoting')
    const full = ready(
      session({
        selected: artifact,
        turns: Array.from({ length: 20 }, (_, i) => ({
          id: String(i),
          request: '말',
          reply: '답',
          jobId: String(i),
          status: 'done',
        })),
      }),
    )
    expect(move(full, { type: 'quote', scopeKey, command: command(full, 'refine') })).toBe(full)
  })
  it('clears an unknown start key only when durable state confirms admitted work and permits publication recovery', () => {
    const base = ready(session({ selected: artifact }))
    const quoted = move(base, { type: 'quote', scopeKey, command: command(base, 'refine') })
    const confirmed = move(quoted, {
      type: 'quoted',
      scopeKey,
      operation: quoted.operation,
      estimate: { free: true },
    })
    const pending = move(confirmed, { type: 'begin', scopeKey, phase: 'starting' })
    const failed = move(pending, {
      type: 'failed',
      scopeKey,
      operation: pending.operation,
      failure: { reason: 'UNKNOWN_FAILURE', params: {} },
    })
    const admitted = move(failed, {
      type: 'hydrate',
      scopeKey,
      session: session({ revision: 2, phase: 'refining', selected: artifact, activeJobId: 'job' }),
    })
    expect(admitted.phase).toBe('active')
    expect(admitted.command).toBeUndefined()
    const saving = ready(session({ phase: 'saving', selected: artifact }))
    expect(move(saving, { type: 'begin', scopeKey, phase: 'saving' }).phase).toBe('saving')
  })
})
