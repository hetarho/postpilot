import { createActor, waitFor } from 'xstate'
import { describe, expect, it, vi } from 'vitest'
import { emptyVoice, type VoiceProfile } from '@/entities/voice'
import { analysisConfirmMachine } from './analysis-confirm-machine'
const scopeKey = 'alice/voice'
const model = { providerId: 'stub', modelId: 'selected' }
const profile: VoiceProfile = {
  voice: emptyVoice(),
  made: false,
  readiness: { percent: 100, sentences: 60, needed: 60, missingParts: [] },
  samples: [],
  activeJobId: '',
  hasPrevious: false,
  notice: { kind: 'none', count: 0 },
}
function start(
  patch: {
    analyze?: (ref: typeof model) => Promise<string>
    read?: () => Promise<VoiceProfile>
  } = {},
) {
  const runtime = {
    current: {
      estimate: vi.fn(async () => ({ free: false, credits: 4 })),
      analyze: vi.fn(async () => 'job'),
      read: vi.fn(async () => profile),
      ...patch,
    },
  }
  const actor = createActor(analysisConfirmMachine, { input: { scopeKey, runtime } }).start()
  return { actor, runtime }
}
describe('explicit analysis estimate and confirmation', () => {
  it('estimates once, cancels freely, and admits only the frozen model on confirmation', async () => {
    const { actor, runtime } = start()
    actor.send({ type: 'ESTIMATE', scopeKey, model, versionKey: 'v1' })
    await waitFor(actor, (s) => s.matches('quoted'))
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    actor.send({ type: 'CANCEL', scopeKey })
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    actor.send({ type: 'ESTIMATE', scopeKey, model, versionKey: 'v1' })
    await waitFor(actor, (s) => s.matches('quoted'))
    actor.send({ type: 'CONFIRM', scopeKey: 'bob/voice', versionKey: 'v1', available: true })
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    actor.send({ type: 'CONFIRM', scopeKey, versionKey: 'v1', available: true })
    actor.send({ type: 'CONFIRM', scopeKey, versionKey: 'v1', available: true })
    await waitFor(actor, (s) => s.matches('started'))
    expect(runtime.current.analyze).toHaveBeenCalledExactlyOnceWith(model)
    actor.stop()
  })
  it('invalidates a quote when source content changes before confirmation', async () => {
    const { actor, runtime } = start()
    actor.send({ type: 'ESTIMATE', scopeKey, model, versionKey: 'v1' })
    await waitFor(actor, (s) => s.matches('quoted'))
    actor.send({ type: 'FACTS', scopeKey, versionKey: 'v2', available: true })
    actor.send({ type: 'CONFIRM', scopeKey, versionKey: 'v2', available: true })
    expect(actor.getSnapshot().matches('idle')).toBe(true)
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    actor.stop()
  })
  it('reads an uncertain admission and resumes the confirmed job without replaying paid work', async () => {
    const analyze = vi.fn(async (): Promise<string> => {
      throw new Error('lost response')
    })
    const read = vi.fn(async () => ({ ...profile, activeJobId: 'admitted' }))
    const { actor } = start({ analyze, read })
    actor.send({ type: 'ESTIMATE', scopeKey, model, versionKey: 'v1' })
    await waitFor(actor, (s) => s.matches('quoted'))
    actor.send({ type: 'CONFIRM', scopeKey, versionKey: 'v1', available: true })
    await waitFor(actor, (s) => s.matches('uncertain'))
    actor.send({ type: 'CONFIRM', scopeKey, versionKey: 'v1', available: true })
    actor.send({ type: 'RECHECK', scopeKey })
    await waitFor(actor, (s) => s.matches('started'))
    expect(actor.getSnapshot().context.jobId).toBe('admitted')
    expect(analyze).toHaveBeenCalledOnce()
    expect(read).toHaveBeenCalledOnce()
    actor.stop()
  })
})
