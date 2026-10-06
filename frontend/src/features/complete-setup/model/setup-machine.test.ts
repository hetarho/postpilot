import { describe, expect, it } from 'vitest'
import {
  emptySetupProgress,
  initialSetupState,
  setupTransition as move,
  SETUP_FORMS,
  type SetupState,
} from './setup-machine'
const ownerId = 'alice'
const hydrate = (missing = [...SETUP_FORMS]): SetupState =>
  move(initialSetupState(ownerId), {
    ownerId,
    type: 'hydrate',
    missing,
    progress: emptySetupProgress(),
  })
const first = () => move(hydrate(), { ownerId, type: 'next', step: 'welcome', confirmed: true })

describe('guarded setup lifecycle', () => {
  it('omits configured steps and requires an explicit start', () => {
    const state = hydrate(['post-template'])
    expect(state.step).toBe('welcome')
    expect(state.plan).toEqual(['post-template'])
    expect(move(state, { ownerId, type: 'next', step: 'welcome', confirmed: true }).step).toBe(
      'post-template',
    )
    expect(hydrate([]).step).toBe('ready')
  })
  it('rejects unknown owners, obsolete steps, premature finish and unconfirmed completion', () => {
    const state = first()
    expect(move(state, { ownerId: 'bob', type: 'skip', step: 'models' })).toBe(state)
    expect(move(state, { ownerId, type: 'next', step: 'voice', confirmed: true })).toBe(state)
    expect(move(state, { ownerId, type: 'next', step: 'models', confirmed: false })).toBe(state)
    expect(move(state, { ownerId, type: 'finish', target: '/' })).toBe(state)
  })
  it('allows exactly one operation and rejects duplicate submissions, stale responses and navigation while pending', () => {
    const state = move(first(), { ownerId, type: 'begin', step: 'models' })
    expect(state.phase).toBe('saving')
    expect(move(state, { ownerId, type: 'begin', step: 'models' })).toBe(state)
    expect(move(state, { ownerId, type: 'skip', step: 'models' })).toBe(state)
    expect(move(state, { ownerId, type: 'back' })).toBe(state)
    expect(move(state, { ownerId, type: 'defer', target: '/' })).toBe(state)
    expect(
      move(state, {
        ownerId,
        type: 'success',
        step: 'voice',
        operation: state.operation,
        complete: true,
      }),
    ).toBe(state)
    expect(
      move(state, {
        ownerId,
        type: 'success',
        step: 'models',
        operation: state.operation - 1,
        complete: true,
      }),
    ).toBe(state)
    expect(
      move(state, {
        ownerId,
        type: 'success',
        step: 'models',
        operation: state.operation,
        complete: false,
      }).step,
    ).toBe('models')
    expect(
      move(state, {
        ownerId,
        type: 'success',
        step: 'models',
        operation: state.operation,
        complete: true,
      }).step,
    ).toBe('voice')
  })
  it('retains a failed step and issues a fresh operation identity when retrying', () => {
    const pending = move(first(), { ownerId, type: 'begin', step: 'models' })
    const failed = move(pending, {
      ownerId,
      type: 'failure',
      step: 'models',
      operation: pending.operation,
    })
    expect(failed).toMatchObject({ phase: 'failed', step: 'models' })
    const retry = move(failed, { ownerId, type: 'begin', step: 'models' })
    expect(retry.operation).toBe(pending.operation + 1)
    expect(
      move(retry, {
        ownerId,
        type: 'success',
        step: 'models',
        operation: pending.operation,
        complete: true,
      }),
    ).toBe(retry)
  })
  it('keeps analysis running until confirmed completion and never advances on navigation', () => {
    const voice = move(hydrate(['voice']), {
      ownerId,
      type: 'next',
      step: 'welcome',
      confirmed: true,
    })
    const saving = move(voice, { ownerId, type: 'begin', step: 'voice' })
    const running = move(saving, {
      ownerId,
      type: 'running',
      step: 'voice',
      operation: saving.operation,
    })
    expect(running.phase).toBe('running')
    expect(move(running, { ownerId, type: 'next', step: 'voice', confirmed: true })).toBe(running)
    expect(move(running, { ownerId, type: 'skip', step: 'voice' })).toBe(running)
    expect(
      move(running, {
        ownerId,
        type: 'success',
        step: 'voice',
        operation: running.operation,
        complete: false,
      }),
    ).toMatchObject({ phase: 'editing', step: 'voice' })
  })
  it('resumes only an eligible missing step, respects explicit skips and preserves saved work on Back', () => {
    const progress = {
      ...emptySetupProgress(),
      skipped: ['models' as const],
      resume: 'voice' as const,
    }
    const resumed = move(initialSetupState(ownerId), {
      ownerId,
      type: 'hydrate',
      missing: [...SETUP_FORMS],
      progress,
    })
    expect(resumed.step).toBe('voice')
    expect(move(resumed, { ownerId, type: 'back' }).step).toBe('welcome')
    const next = move(resumed, { ownerId, type: 'next', step: 'voice', confirmed: true })
    expect(next.step).toBe('post-template')
    expect(move(next, { ownerId, type: 'back' }).resolved).toContain('voice')
    expect(
      move(initialSetupState(ownerId), {
        ownerId,
        type: 'hydrate',
        missing: ['clip-template'],
        progress,
      }).step,
    ).toBe('welcome')
  })
  it('finishes exactly once after all explicit skips and sanitizes a persisted destination', () => {
    let state = first()
    for (const step of SETUP_FORMS) state = move(state, { ownerId, type: 'skip', step })
    expect(state.step).toBe('ready')
    const finished = move(state, { ownerId, type: 'finish', target: '/clips/new' })
    expect(finished).toMatchObject({ phase: 'completed', target: '/clips/new' })
    expect(move(finished, { ownerId, type: 'finish', target: '/posts/new' })).toBe(finished)
    const poisoned = move(initialSetupState(ownerId), {
      ownerId,
      type: 'hydrate',
      missing: ['voice'],
      progress: { ...emptySetupProgress(), target: '//evil.test' as '/' },
    })
    expect(poisoned.target).toBe('/')
  })
  it('can explicitly defer an unknown or failed initial read without assuming any configuration', () => {
    expect(move(initialSetupState(ownerId), { ownerId, type: 'defer', target: '/' })).toMatchObject(
      { phase: 'completed', step: 'welcome', resolved: [] },
    )
  })
})
