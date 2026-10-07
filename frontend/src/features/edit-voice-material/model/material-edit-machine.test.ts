import { Code } from '@connectrpc/connect'
import { createActor, waitFor } from 'xstate'
import { describe, expect, it, vi } from 'vitest'
import type { VoiceSampleDetail } from '@/entities/voice'
import { connectAppError } from '@/test/app-error'
import { materialEditMachine, type MaterialEditServices } from './material-edit-machine'

const scopeKey = 'alice/voice/post'
const body = '내가 직접 적은 원래 글. '.repeat(30)
const detail: VoiceSampleDetail = {
  sample: {
    id: 'post',
    kind: 'post',
    label: '제목',
    promptKey: '',
    hasPhoto: false,
    chars: body.length,
    createdAt: '',
    contentRevision: 1n,
  },
  body,
  photoUrl: '',
  photoWidth: 0,
  photoHeight: 0,
}
function start(
  patch: Partial<MaterialEditServices> = {},
  baseline = detail,
  photoRequired = false,
) {
  const services = {
    update: vi.fn(async () => ({ ...baseline.sample, contentRevision: 2n })),
    read: vi.fn(async () => ({
      ...baseline,
      body: '다른 곳의 최신 글. '.repeat(30),
      sample: { ...baseline.sample, contentRevision: 2n },
    })),
    upload: vi.fn(async () => ({ uploadId: 'upload', width: 100, height: 100 })),
    ...patch,
  }
  const actor = createActor(materialEditMachine, {
    input: {
      scopeKey,
      baseline,
      photoRequired,
      blocked: false,
      minimumPostChars: 200,
      services,
      requestKey: vi.fn(() => 'intent'),
    },
  }).start()
  return { actor, services }
}
describe('material editing lifecycle', () => {
  it('keeps Cancel and invalid source drafts free of writes and guards another owner', () => {
    const { actor, services } = start()
    actor.send({ type: 'CHANGE', scopeKey, patch: { body: '짧은 글' } })
    actor.send({ type: 'SAVE', scopeKey })
    actor.send({ type: 'SAVE', scopeKey: 'bob/voice/post' })
    actor.send({ type: 'CANCEL', scopeKey })
    expect(actor.getSnapshot().matches('cancelled')).toBe(true)
    expect(services.update).not.toHaveBeenCalled()
    expect(services.upload).not.toHaveBeenCalled()
  })
  it('saves exactly one CAS intent without inventing a photo replacement', async () => {
    const { actor, services } = start()
    actor.send({ type: 'CHANGE', scopeKey, patch: { label: '새 제목' } })
    actor.send({ type: 'SAVE', scopeKey })
    actor.send({ type: 'SAVE', scopeKey })
    await waitFor(actor, (s) => s.matches('saved'))
    expect(services.update).toHaveBeenCalledOnce()
    expect(services.update).toHaveBeenCalledWith(
      { sampleId: 'post', expectedContentRevision: 1n, operationKey: 'intent', label: '새 제목' },
      expect.any(AbortSignal),
    )
    expect(services.upload).not.toHaveBeenCalled()
    actor.stop()
  })
  it('preserves unsaved text on conflict, reads the latest revision, then requires a new Save', async () => {
    const update = vi
      .fn<MaterialEditServices['update']>()
      .mockRejectedValueOnce(connectAppError('VOICE_SAMPLE_REVISION_CONFLICT', Code.Aborted))
      .mockResolvedValueOnce({ ...detail.sample, contentRevision: 3n })
    const { actor, services } = start({ update })
    const draft = '내가 지금 고친 글. '.repeat(30)
    actor.send({ type: 'CHANGE', scopeKey, patch: { body: draft } })
    actor.send({ type: 'SAVE', scopeKey })
    await waitFor(actor, (s) => s.matches('conflict'))
    expect(actor.getSnapshot().context.draft.body).toBe(draft)
    actor.send({ type: 'RELOAD', scopeKey })
    await waitFor(actor, (s) => s.matches('editing'))
    expect(actor.getSnapshot().context.draft.body).toBe(draft)
    expect(actor.getSnapshot().context.baseline.sample.contentRevision).toBe(2n)
    expect(services.update).toHaveBeenCalledOnce()
    actor.send({ type: 'SAVE', scopeKey })
    await waitFor(actor, (s) => s.matches('saved'))
    expect(update.mock.calls[1]?.[0].expectedContentRevision).toBe(2n)
    actor.stop()
  })
  it('reuses the uploaded photo receipt and operation key after an unconfirmed save', async () => {
    const answer: VoiceSampleDetail = {
      ...detail,
      sample: {
        ...detail.sample,
        id: 'answer',
        kind: 'answer',
        promptKey: 'photo',
        hasPhoto: true,
      },
      body: '원래 답변',
      photoUrl: 'saved.jpg',
    }
    const update = vi
      .fn<MaterialEditServices['update']>()
      .mockRejectedValueOnce(new Error('lost acknowledgement'))
      .mockResolvedValueOnce({ ...answer.sample, contentRevision: 2n })
    const { actor, services } = start({ update }, answer, true)
    actor.send({
      type: 'CHANGE',
      scopeKey,
      patch: {
        photo: {
          mode: 'replace',
          photo: { blob: new Blob(['photo']), width: 100, height: 100 },
          preview: 'blob:photo',
        },
      },
    })
    actor.send({ type: 'SAVE', scopeKey })
    await waitFor(actor, (s) => s.matches('saveFailed'))
    actor.send({ type: 'RETRY', scopeKey })
    await waitFor(actor, (s) => s.matches('saved'))
    expect(services.upload).toHaveBeenCalledOnce()
    expect(update.mock.calls[0]?.[0]).toEqual(update.mock.calls[1]?.[0])
    expect(update.mock.calls[1]?.[0].photo).toEqual({ uploadId: 'upload', width: 100, height: 100 })
    actor.stop()
  })
  it('cannot remove a required photo and drops late owner acknowledgements', async () => {
    const answer: VoiceSampleDetail = {
      ...detail,
      sample: { ...detail.sample, kind: 'answer', promptKey: 'photo', hasPhoto: true },
      body: '원래 답변',
    }
    let resolve!: (value: typeof detail.sample) => void
    const update = vi.fn(
      () =>
        new Promise<typeof detail.sample>((done) => {
          resolve = done
        }),
    )
    const { actor } = start({ update }, answer, true)
    actor.send({ type: 'CHANGE', scopeKey, patch: { photo: { mode: 'remove' } } })
    actor.send({ type: 'SAVE', scopeKey })
    expect(update).not.toHaveBeenCalled()
    actor.send({ type: 'CHANGE', scopeKey, patch: { photo: { mode: 'keep' }, body: '바꾼 답변' } })
    actor.send({ type: 'SAVE', scopeKey })
    actor.stop()
    resolve({ ...answer.sample, contentRevision: 2n })
    await Promise.resolve()
    await Promise.resolve()
    expect(actor.getSnapshot().context.saved).toBeUndefined()
  })
})
