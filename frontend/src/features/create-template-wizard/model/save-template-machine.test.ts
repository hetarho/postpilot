import { createActor } from 'xstate'
import { expect, it, vi } from 'vitest'
import { saveTemplateMachine } from './save-template-machine'

const fields = { name: '내 구성', description: '', titleArea: '', body: '<write>방문</write>' }
function deferred() {
  let resolve!: (id: string) => void
  const promise = new Promise<string>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
it('admits one owned save, freezes it and confirms the saved identity once', async () => {
  const gate = deferred()
  const save = vi.fn(() => gate.promise)
  const onSaved = vi.fn()
  const beforeSave = vi.fn(() => true)
  const actor = createActor(saveTemplateMachine, {
    input: { ownerId: 'alice', save, onSaved, beforeSave },
  }).start()
  actor.send({ type: 'save', ownerId: 'bob', fields })
  expect(save).not.toHaveBeenCalled()
  const input = { ...fields }
  actor.send({ type: 'save', ownerId: 'alice', fields: input })
  actor.send({ type: 'save', ownerId: 'alice', fields })
  input.name = 'late change'
  expect(save).toHaveBeenCalledTimes(1)
  expect(save.mock.calls[0]).toEqual([fields])
  expect(beforeSave).toHaveBeenCalledTimes(1)
  gate.resolve('saved-id')
  await vi.waitFor(() => expect(onSaved).toHaveBeenCalledWith('saved-id'))
  actor.send({ type: 'save', ownerId: 'alice', fields })
  expect(save).toHaveBeenCalledTimes(1)
  actor.stop()
})
it('retains the failed draft for explicit retry and fences results after stop', async () => {
  const gate = deferred()
  const save = vi
    .fn()
    .mockRejectedValueOnce(new Error('offline'))
    .mockImplementationOnce(() => gate.promise)
  const onSaved = vi.fn()
  const onFailure = vi.fn()
  const actor = createActor(saveTemplateMachine, {
    input: { ownerId: 'alice', save, onSaved, onFailure },
  }).start()
  actor.send({ type: 'save', ownerId: 'alice', fields })
  await vi.waitFor(() => expect(onFailure).toHaveBeenCalledTimes(1))
  expect(actor.getSnapshot().context.fields).toEqual(fields)
  actor.send({ type: 'save', ownerId: 'alice', fields })
  actor.stop()
  gate.resolve('old-owner-result')
  await gate.promise
  await Promise.resolve()
  expect(onSaved).not.toHaveBeenCalled()
  expect(save).toHaveBeenCalledTimes(2)
})
it('issues no canonical write when the setup actor declines admission', async () => {
  const save = vi.fn().mockResolvedValue('saved')
  const onSaved = vi.fn()
  const onFailure = vi.fn()
  const actor = createActor(saveTemplateMachine, {
    input: {
      ownerId: 'alice',
      save,
      onSaved,
      onFailure,
      beforeSave: () => false,
    },
  }).start()
  actor.send({ type: 'save', ownerId: 'alice', fields })
  await vi.waitFor(() => expect(onFailure).toHaveBeenCalledTimes(1))
  expect(save).not.toHaveBeenCalled()
  expect(onSaved).not.toHaveBeenCalled()
  actor.stop()
})
