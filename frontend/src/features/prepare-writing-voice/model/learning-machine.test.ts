import { createActor, waitFor } from 'xstate'
import { describe, expect, it, vi } from 'vitest'
import { emptyVoice, type VoiceProfile } from '@/entities/voice'
import { learningMachine, type LearningServices } from './learning-machine'
const voice = { ...emptyVoice(), id: 'voice', name: '나의 말투' }
const profile = (patch: Partial<VoiceProfile> = {}): VoiceProfile => ({
  voice,
  made: false,
  readiness: {
    percent: 100,
    sentences: 60,
    needed: 60,
    missingParts: [],
    answeredQuestions: 10,
    requiredQuestions: 10,
  },
  samples: [],
  activeJobId: '',
  hasPrevious: false,
  notice: { kind: 'none', count: 0 },
  ...patch,
})
function start(options: Partial<LearningServices> = {}) {
  const runtime = {
    current: {
      create: vi.fn(async () => voice),
      estimate: vi.fn(async () => ({ free: false, credits: 3 })),
      analyze: vi.fn(async () => 'current-job'),
      confirm: vi.fn(async () => ({ ...voice, made: true, isDefault: true })),
      read: vi.fn(async () => profile()),
      ...options,
    },
  }
  const actor = createActor(learningMachine, {
    input: {
      ownerId: 'alice',
      initialVoiceId: 'voice',
      initialMethod: 'questions',
      name: '나의 말투',
      runtime,
    },
  }).start()
  actor.send({ ownerId: 'alice', type: 'PROFILE', profile: profile() })
  actor.send({ ownerId: 'alice', type: 'REVIEW' })
  return { actor, runtime }
}
describe('personal learning actor', () => {
  it('keeps collecting/review navigation free of analysis and guards other owners', () => {
    const { actor, runtime } = start()
    expect(actor.getSnapshot().matches({ personal: 'review' })).toBe(true)
    actor.send({ ownerId: 'bob', type: 'ANALYZE', model: { providerId: 'stub', modelId: 'model' } })
    actor.send({ ownerId: 'alice', type: 'BACK' })
    expect(actor.getSnapshot().matches({ personal: { collecting: 'questions' } })).toBe(true)
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    expect(runtime.current.create).not.toHaveBeenCalled()
    actor.stop()
  })
  it('admits only one analysis and ignores a stale job failure', async () => {
    const { actor, runtime } = start()
    const event = {
      ownerId: 'alice',
      type: 'ANALYZE' as const,
      model: { providerId: 'stub', modelId: 'model' },
    }
    actor.send({ ...event, type: 'ESTIMATE' })
    await waitFor(actor, (s) => s.matches({ personal: 'quoted' }))
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    actor.send(event)
    actor.send(event)
    await waitFor(actor, (s) => s.matches({ personal: { analyzing: 'watching' } }))
    expect(runtime.current.analyze).toHaveBeenCalledOnce()
    actor.send({ ownerId: 'alice', type: 'JOB_FAILED', jobId: 'old-job' })
    expect(actor.getSnapshot().matches({ personal: { analyzing: 'watching' } })).toBe(true)
    actor.send({ ownerId: 'alice', type: 'JOB_FAILED', jobId: 'current-job' })
    expect(actor.getSnapshot().matches('failure')).toBe(true)
    actor.stop()
  })
  it('requires a new quote after source content changes, while label-only edits keep the prepared quote', async () => {
    const { actor, runtime } = start()
    const model = { providerId: 'stub', modelId: 'model' }
    const sample = {
      id: 'source',
      kind: 'post' as const,
      label: '제목',
      promptKey: '',
      hasPhoto: false,
      chars: 200,
      createdAt: '',
      contentRevision: 1n,
    }
    actor.send({ ownerId: 'alice', type: 'PROFILE', profile: profile({ samples: [sample] }) })
    actor.send({ ownerId: 'alice', type: 'ESTIMATE', model })
    await waitFor(actor, (s) => s.matches({ personal: 'quoted' }))
    actor.send({
      ownerId: 'alice',
      type: 'PROFILE',
      profile: profile({ samples: [{ ...sample, label: '새 제목' }] }),
    })
    expect(actor.getSnapshot().matches({ personal: 'quoted' })).toBe(true)
    actor.send({
      ownerId: 'alice',
      type: 'PROFILE',
      profile: profile({ samples: [{ ...sample, contentRevision: 2n }] }),
    })
    actor.send({ ownerId: 'alice', type: 'ANALYZE', model })
    expect(actor.getSnapshot().matches({ personal: 'review' })).toBe(true)
    expect(runtime.current.analyze).not.toHaveBeenCalled()
    expect(runtime.current.estimate).toHaveBeenCalledOnce()
    actor.stop()
  })
  it.each(['RETRY', 'BACK'] as const)(
    'reconciles an uncertain analysis through %s without replaying it',
    async (action) => {
      const read = vi.fn(async () => profile({ made: true, voice: { ...voice, made: true } }))
      const analyze = vi.fn(async (): Promise<string> => {
        throw new Error('unknown delivery')
      })
      const { actor } = start({ read, analyze })
      const model = { providerId: 'stub', modelId: 'model' }
      actor.send({ ownerId: 'alice', type: 'ESTIMATE', model })
      await waitFor(actor, (s) => s.matches({ personal: 'quoted' }))
      actor.send({ ownerId: 'alice', type: 'ANALYZE', model })
      await waitFor(actor, (s) => s.matches('failure'))
      actor.send({ ownerId: 'alice', type: action })
      await waitFor(actor, (s) => s.matches({ personal: 'confirmed' }))
      expect(read).toHaveBeenCalledOnce()
      expect(analyze).toHaveBeenCalledOnce()
      actor.stop()
    },
  )
  it('accepts matching published or active analysis facts after an uncertain response without new work', async () => {
    const analyze = vi.fn(async (): Promise<string> => {
      throw new Error('unknown delivery')
    })
    const { actor } = start({ analyze })
    const model = { providerId: 'stub', modelId: 'model' }
    actor.send({ ownerId: 'alice', type: 'ESTIMATE', model })
    await waitFor(actor, (s) => s.matches({ personal: 'quoted' }))
    actor.send({ ownerId: 'alice', type: 'ANALYZE', model })
    await waitFor(actor, (s) => s.matches('failure'))
    actor.send({
      ownerId: 'alice',
      type: 'PROFILE',
      profile: profile({ activeJobId: 'admitted-job' }),
    })
    expect(actor.getSnapshot().matches({ personal: { analyzing: 'watching' } })).toBe(true)
    expect(actor.getSnapshot().context.jobId).toBe('admitted-job')
    actor.send({
      ownerId: 'alice',
      type: 'PROFILE',
      profile: profile({ made: true, voice: { ...voice, made: true } }),
    })
    expect(actor.getSnapshot().matches({ personal: 'confirmed' })).toBe(true)
    expect(analyze).toHaveBeenCalledOnce()
    actor.stop()
  })
  it('drops a create acknowledgement after its owner lifetime stops', async () => {
    let release!: (value: typeof voice) => void
    const create = vi.fn(
      () =>
        new Promise<typeof voice>((resolve) => {
          release = resolve
        }),
    )
    const runtime = {
      current: {
        create,
        analyze: vi.fn(async () => ''),
        confirm: vi.fn(async () => voice),
        read: vi.fn(async () => profile()),
      },
    }
    const actor = createActor(learningMachine, {
      input: { ownerId: 'alice', name: '나의 말투', runtime },
    }).start()
    actor.send({ ownerId: 'alice', type: 'CHOOSE', method: 'paste' })
    expect(create).toHaveBeenCalledOnce()
    actor.stop()
    release(voice)
    await Promise.resolve()
    await Promise.resolve()
    expect(actor.getSnapshot().context.voiceId).toBe('')
    expect(runtime.current.analyze).not.toHaveBeenCalled()
  })
})
