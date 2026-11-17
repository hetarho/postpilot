import { describe, expect, it } from 'vitest'
import {
  answerTurnKey,
  initialQuestionnaireState,
  questionnaireCanSkip,
  questionnaireCurrentKey,
  questionnaireSavedCount,
  questionnaireTransition,
  type QuestionnaireEvent,
  type QuestionnaireState,
} from './questionnaire-machine'

const identity = { ownerId: 'alice', voiceId: 'personal' }
const catalog = Array.from({ length: 240 }, (_, index) => ({
  key: `question-${index}`,
  photo: index === 0,
  starter: index >= 1 && index <= 10,
}))
const send = (state: QuestionnaireState, event: Omit<QuestionnaireEvent, 'ownerId' | 'voiceId'>) =>
  questionnaireTransition(state, { ...identity, ...event } as QuestionnaireEvent)
/** The imagined-situations path: `생각나는 게 없어요` on a first turn, `다른 장면으로 답하기` later. */
function toSituations(state: QuestionnaireState) {
  return send(state, { type: 'situations' })
}
function start(saved: string[] = [], made = false) {
  return toSituations(
    questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
      ...identity,
      type: 'hydrate',
      catalog,
      saved,
      made,
    }),
  )
}
function save(state: QuestionnaireState, body = '평소처럼 써 봤어요.') {
  const key = questionnaireCurrentKey(state)
  const drafted = send(state, { type: 'draft', key, body } as QuestionnaireEvent)
  const pending = send(drafted, { type: 'begin', key } as QuestionnaireEvent)
  return send(pending, { type: 'success', key, operation: pending.operation } as QuestionnaireEvent)
}

describe('ten-question session', () => {
  it('prioritizes photo-free starters and stops after ten confirmed answers in a large catalog', () => {
    let state = start()
    expect(questionnaireCurrentKey(state)).toBe('question-1')
    for (let index = 0; index < 9; index++) state = save(state)
    expect(state.phase).toBe('answering')
    expect(questionnaireSavedCount(state)).toBe(9)
    state = save(state)
    expect(state.phase).toBe('complete')
    expect(state.completion).toBe('ten')
    expect(questionnaireSavedCount(state)).toBe(10)
    expect(state.saved).toHaveLength(10)
    expect(state.saved).not.toContain('question-0')
  })

  it('resumes a personal bootstrap from unique saved keys without asking those questions again', () => {
    let state = start(['question-1', 'question-1', 'question-3', 'question-7'])
    expect(questionnaireSavedCount(state)).toBe(3)
    expect(questionnaireCurrentKey(state)).toBe('question-2')
    for (let index = 0; index < 7; index++) state = save(state)
    expect(state.phase).toBe('complete')
    expect(state.saved).toHaveLength(10)
  })

  it('starts a made voice on ten new questions and keeps earlier answers through another batch', () => {
    const previous = [
      'question-0',
      ...Array.from({ length: 10 }, (_, index) => `question-${index + 1}`),
    ]
    let state = start(previous, true)
    expect(questionnaireSavedCount(state)).toBe(0)
    expect(questionnaireCurrentKey(state)).toBe('question-11')
    for (let index = 0; index < 10; index++) state = save(state)
    expect(state.saved.slice(0, previous.length)).toEqual(previous)
    state = toSituations(send(state, { type: 'new-session' }))
    expect(questionnaireCurrentKey(state)).toBe('question-21')
    expect(questionnaireSavedCount(state)).toBe(0)
    for (let index = 0; index < 10; index++) state = save(state)
    expect(state.saved).toHaveLength(previous.length + 20)
  })

  it('skips by replacing a question without counting it or deleting its draft', () => {
    let state = start()
    state = send(state, {
      type: 'draft',
      key: 'question-1',
      body: '이 문장은 아직 고민 중이에요.',
    } as QuestionnaireEvent)
    state = send(state, { type: 'skip' })
    expect(questionnaireCurrentKey(state)).toBe('question-2')
    expect(questionnaireSavedCount(state)).toBe(0)
    expect(state.saved).toEqual([])
    expect(state.drafts['question-1']).toBe('이 문장은 아직 고민 중이에요.')
    expect(state.skipped).toEqual(['question-1'])
  })

  it('preserves drafts on failure and Back while an explicit rewrite adds no completed question', () => {
    let state = start()
    state = save(state, '첫 번째 답이에요.')
    state = send(state, {
      type: 'draft',
      key: 'question-2',
      body: '두 번째 답을 쓰는 중이에요.',
    } as QuestionnaireEvent)
    state = send(state, { type: 'back' })
    expect(state.drafts[questionnaireCurrentKey(state)]).toBe('첫 번째 답이에요.')
    state = save(state, '첫 번째 답을 고쳤어요.')
    expect(questionnaireSavedCount(state)).toBe(1)
    expect(questionnaireCurrentKey(state)).toBe('question-2')
    expect(state.drafts['question-2']).toBe('두 번째 답을 쓰는 중이에요.')
    state = send(state, { type: 'begin', key: 'question-2' } as QuestionnaireEvent)
    state = send(state, {
      type: 'failure',
      key: 'question-2',
      operation: state.operation,
    } as QuestionnaireEvent)
    expect(state.phase).toBe('failed')
    expect(state.drafts['question-2']).toBe('두 번째 답을 쓰는 중이에요.')
    expect(questionnaireSavedCount(state)).toBe(1)
  })

  it('replaces a skipped closing with another closing so the starter keeps every writing part', () => {
    const questions = [
      { key: 'opening', photo: false, starter: true, part: 'opening' as const },
      { key: 'closing', photo: false, starter: true, part: 'closing' as const },
      { key: 'description', photo: false, starter: true, part: 'description' as const },
      { key: 'another-closing', photo: false, part: 'closing' as const },
    ]
    let state = toSituations(
      questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
        ...identity,
        type: 'hydrate',
        catalog: questions,
        saved: ['opening'],
        made: false,
      }),
    )
    expect(questionnaireCurrentKey(state)).toBe('closing')
    state = send(state, { type: 'skip' })
    expect(questionnaireCurrentKey(state)).toBe('another-closing')
    expect(questionnaireSavedCount(state)).toBe(1)
  })

  it('locks duplicate submits, navigation and late responses behind owner, voice, key and operation guards', () => {
    const pending = send(start(), { type: 'begin', key: 'question-1' } as QuestionnaireEvent)
    expect(send(pending, { type: 'begin', key: 'question-1' } as QuestionnaireEvent)).toBe(pending)
    for (const type of ['back', 'skip', 'new-session'] as const)
      expect(send(pending, { type })).toBe(pending)
    const valid = {
      ...identity,
      type: 'success',
      key: 'question-1',
      operation: pending.operation,
    } as const
    for (const invalid of [
      { ...valid, ownerId: 'bob' },
      { ...valid, voiceId: 'another' },
      { ...valid, key: 'question-2' },
      { ...valid, operation: pending.operation - 1 },
    ])
      expect(questionnaireTransition(pending, invalid)).toBe(pending)
    const confirmed = questionnaireTransition(pending, valid)
    expect(questionnaireTransition(confirmed, valid)).toBe(confirmed)
    expect(questionnaireSavedCount(confirmed)).toBe(1)
  })

  it('ends on catalog exhaustion without rewriting and enters saved-answer review only explicitly', () => {
    let state = toSituations(
      questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
        ...identity,
        type: 'hydrate',
        catalog: catalog.slice(1, 3),
        saved: [],
        made: false,
      }),
    )
    state = save(save(state))
    expect(state.phase).toBe('complete')
    expect(state.completion).toBe('exhausted')
    expect(questionnaireSavedCount(state)).toBe(2)
    state = send(state, { type: 'review', key: 'question-1' } as QuestionnaireEvent)
    expect(state.reviewing).toBe(true)
    state = save(state, '답을 직접 고쳤어요.')
    expect(state.phase).toBe('complete')
    expect(questionnaireSavedCount(state)).toBe(2)
  })

  it('uses the server valid-answer count when some legacy saved keys are not counted', () => {
    const state = toSituations(
      questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
        ...identity,
        type: 'hydrate',
        catalog,
        saved: ['question-1', 'question-2'],
        made: false,
        answeredQuestions: 1,
      }),
    )
    expect(questionnaireSavedCount(state)).toBe(1)
    expect(questionnaireCurrentKey(state)).toBe('question-3')
  })

  it('completes missing opening and closing coverage for an owner with eight legacy description answers', () => {
    const questions = [
      ...Array.from({ length: 8 }, (_, index) => ({
        key: `legacy-${index}`,
        photo: false,
        part: 'description' as const,
      })),
      { key: 'opening-1', photo: false, starter: true, part: 'opening' as const },
      { key: 'opening-2', photo: false, starter: true, part: 'opening' as const },
      { key: 'description', photo: false, starter: true, part: 'description' as const },
      { key: 'closing', photo: false, starter: true, part: 'closing' as const },
    ]
    let state = toSituations(
      questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
        ...identity,
        type: 'hydrate',
        catalog: questions,
        saved: questions.slice(0, 8).map((question) => question.key),
        made: false,
        answeredQuestions: 8,
        missingParts: ['opening', 'closing'],
      }),
    )
    expect(questionnaireCurrentKey(state)).toBe('opening-1')
    state = save(state)
    expect(questionnaireCurrentKey(state)).toBe('closing')
    state = save(state)
    expect(state.phase).toBe('complete')
    expect(state.coveredParts).toEqual(['description', 'opening', 'closing'])
    expect(questionnaireSavedCount(state)).toBe(10)
  })
})

describe('experience thread (VOICE-78)', () => {
  const thread = [
    ...[
      'opening',
      'opening',
      'description',
      'description',
      'description',
      'description',
      'description',
      'closing',
      'closing',
    ].map((part, index) => ({
      key: `thread_food_${part}_${index}`,
      photo: false,
      part: part as 'opening' | 'description' | 'closing',
      experienceKind: 'food' as const,
    })),
    ...catalog.slice(1, 12),
  ]
  const food = { id: 'e1', kind: 'food' as const }
  const hydrate = (saved: string[] = [], experiences: (typeof food)[] = [], made = false) =>
    questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
      ...identity,
      type: 'hydrate',
      catalog: thread,
      saved,
      made,
      experiences,
    })
  const pick = (state: QuestionnaireState, experience = food) => {
    const starting = send(state, { type: 'start-experience' })
    return send(starting, {
      type: 'experience-started',
      experience,
      operation: starting.operation,
    } as QuestionnaireEvent)
  }
  const turn = (experienceId: string, index: number) =>
    answerTurnKey(experienceId, thread[index].key)

  it('asks for the owner’s own experience first and asks its thread in order', () => {
    let state = hydrate()
    expect(state.phase).toBe('choosing')
    state = pick(state)
    expect(state.phase).toBe('answering')
    const asked: string[] = []
    for (let reply = 0; reply < 5; reply++) {
      asked.push(questionnaireCurrentKey(state))
      state = save(state)
    }
    expect(asked).toEqual([
      turn('e1', 0),
      turn('e1', 2),
      turn('e1', 3),
      turn('e1', 4),
      turn('e1', 7),
    ])
    expect(state.phase).toBe('offer')
    expect(questionnaireSavedCount(state)).toBe(5)
  })

  it('keeps a failed start on the pick and ignores a late or foreign start', () => {
    const starting = send(hydrate(), { type: 'start-experience' })
    expect(starting.phase).toBe('starting')
    const late = { type: 'experience-started', experience: food, operation: starting.operation - 1 }
    expect(send(starting, late as QuestionnaireEvent)).toBe(starting)
    const failed = { type: 'experience-failed', operation: starting.operation }
    expect(send(starting, failed as QuestionnaireEvent).phase).toBe('choosing')
  })

  it('swaps in the next unused question of the same part without counting the skipped one', () => {
    let state = pick(hydrate())
    expect(questionnaireCanSkip(state)).toBe(true)
    state = send(state, { type: 'skip' })
    expect(questionnaireCurrentKey(state)).toBe(turn('e1', 1))
    expect(questionnaireSavedCount(state)).toBe(0)
    expect(questionnaireCanSkip(state)).toBe(false)
    expect(send(state, { type: 'skip' })).toBe(state)
    state = save(state)
    expect(questionnaireCurrentKey(state)).toBe(turn('e1', 2))
  })

  it('offers another experience or the situations after the first one, and ten replies end the chat', () => {
    let state = pick(hydrate())
    for (let reply = 0; reply < 5; reply++) state = save(state)
    expect(state.phase).toBe('offer')
    state = send(state, { type: 'another-experience' })
    expect(state.phase).toBe('choosing')
    state = pick(state, { id: 'e2', kind: 'food' })
    for (let reply = 0; reply < 4; reply++) state = save(state)
    expect(state.phase).toBe('answering')
    state = save(state)
    expect(state.phase).toBe('complete')
    expect(state.completion).toBe('ten')
    expect(questionnaireSavedCount(state)).toBe(10)
    expect(new Set(state.saved).size).toBe(10)
  })

  it('keeps two experiences of the same kind with separate answers', () => {
    let state = pick(hydrate())
    for (let reply = 0; reply < 5; reply++) state = save(state)
    state = pick(send(state, { type: 'another-experience' }), { id: 'e2', kind: 'food' })
    expect(questionnaireCurrentKey(state)).toBe(turn('e2', 0))
    state = save(state)
    expect(state.saved).toContain(turn('e1', 0))
    expect(state.saved).toContain(turn('e2', 0))
    expect(turn('e1', 0)).not.toBe(turn('e2', 0))
  })

  it('moves to the starter situations when the owner has nothing in mind', () => {
    const state = send(hydrate(), { type: 'situations' })
    expect(state.phase).toBe('answering')
    expect(questionnaireCurrentKey(state)).toBe(catalog[1].key)
    expect(questionnaireCanSkip(state)).toBe(true)
  })

  it('resumes an unfinished experience at its next unanswered question', () => {
    const state = hydrate([turn('e1', 0), turn('e1', 2)], [food])
    expect(state.phase).toBe('answering')
    expect(questionnaireCurrentKey(state)).toBe(turn('e1', 3))
    expect(state.experienceId).toBe('e1')
  })

  it('resumes a started experience that has no reply yet', () => {
    expect(questionnaireCurrentKey(hydrate([], [food]))).toBe(turn('e1', 0))
  })

  it('offers another experience first and situations second on a later chat', () => {
    let state = hydrate(
      Array.from({ length: 5 }, (_, i) => turn('e0', [0, 2, 3, 4, 7][i])),
      [{ id: 'e0', kind: 'food' }],
      true,
    )
    expect(state.phase).toBe('offer')
    expect(state.mode).toBe('more')
    state = send(state, { type: 'situations' })
    expect(questionnaireCurrentKey(state)).toBe(catalog[1].key)
  })

  it('opens the saved replies for review from the offer only when there are some', () => {
    expect(send(hydrate([], [], true), { type: 'review-saved' }).phase).toBe('offer')
    const saved = [0, 2, 3, 4, 7].map((index) => turn('e0', index))
    const offer = hydrate(saved, [{ id: 'e0', kind: 'food' }], true)
    expect(offer.phase).toBe('offer')
    const reviewing = send(offer, { type: 'review-saved' })
    expect(reviewing.phase).toBe('complete')
    expect(reviewing.completion).toBe('saved')
  })
})
