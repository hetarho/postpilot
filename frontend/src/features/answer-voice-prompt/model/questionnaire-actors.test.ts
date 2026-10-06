import { createActor } from 'xstate'
import { expect, it } from 'vitest'
import {
  questionnaireMachine,
  questionnaireStateOf,
  questionnaireCurrentKey,
  questionnaireSavedCount,
} from './questionnaire-machine'

const identity = { ownerId: 'alice', voiceId: 'personal' }
const catalog = Array.from({ length: 40 }, (_, i) => ({
  key: String(i),
  photo: false,
  part: (['opening', 'description', 'closing'] as const)[i % 3],
}))
it('counts only confirmed answers through an actual actor and fences duplicate saves and late results', () => {
  const actor = createActor(questionnaireMachine, { input: identity }).start()
  actor.send({ ...identity, type: 'hydrate', catalog, saved: [], made: false })
  const key = questionnaireCurrentKey(questionnaireStateOf(actor.getSnapshot()))
  actor.send({ ...identity, type: 'begin', key })
  const reserved = actor.getSnapshot()
  actor.send({ ...identity, type: 'begin', key })
  actor.send({ ...identity, type: 'skip' })
  actor.send({ ...identity, type: 'back' })
  expect(actor.getSnapshot()).toBe(reserved)
  actor.send({
    ...identity,
    ownerId: 'bob',
    type: 'success',
    key,
    operation: reserved.context.operation,
  })
  actor.send({ ...identity, type: 'success', key, operation: reserved.context.operation - 1 })
  expect(actor.getSnapshot().matches('saving')).toBe(true)
  actor.send({ ...identity, type: 'success', key, operation: reserved.context.operation })
  expect(questionnaireSavedCount(questionnaireStateOf(actor.getSnapshot()))).toBe(1)
  expect(questionnaireCurrentKey(questionnaireStateOf(actor.getSnapshot()))).not.toBe(key)
  actor.stop()
})

it('recovers server readiness, reviews existing answers and starts another batch without inventing saved data', () => {
  const actor = createActor(questionnaireMachine, { input: identity }).start()
  actor.send({
    ...identity,
    type: 'hydrate',
    catalog,
    saved: ['0'],
    made: false,
    answeredQuestions: 10,
    missingParts: [],
  })
  expect(actor.getSnapshot().matches('complete')).toBe(true)
  actor.send({ ...identity, type: 'review', key: '0' })
  actor.send({ ...identity, type: 'back' })
  expect(actor.getSnapshot().matches('complete')).toBe(true)
  actor.send({ ...identity, type: 'new-session' })
  expect(actor.getSnapshot().matches('answering')).toBe(true)
  expect(actor.getSnapshot().context.saved).toEqual(['0'])
  expect(questionnaireSavedCount(questionnaireStateOf(actor.getSnapshot()))).toBe(0)
  actor.stop()
})
