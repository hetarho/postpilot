import { describe, expect, it } from 'vitest'
import {
  initialQuestionnaireState,
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
function start(saved: string[] = [], made = false) {
  return questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
    ...identity,
    type: 'hydrate',
    catalog,
    saved,
    made,
  })
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
    state = send(state, { type: 'new-session' })
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
    let state = questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
      ...identity,
      type: 'hydrate',
      catalog: questions,
      saved: ['opening'],
      made: false,
    })
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
    let state = questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
      ...identity,
      type: 'hydrate',
      catalog: catalog.slice(1, 3),
      saved: [],
      made: false,
    })
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
    const state = questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
      ...identity,
      type: 'hydrate',
      catalog,
      saved: ['question-1', 'question-2'],
      made: false,
      answeredQuestions: 1,
    })
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
    let state = questionnaireTransition(initialQuestionnaireState('alice', 'personal'), {
      ...identity,
      type: 'hydrate',
      catalog: questions,
      saved: questions.slice(0, 8).map((question) => question.key),
      made: false,
      answeredQuestions: 8,
      missingParts: ['opening', 'closing'],
    })
    expect(questionnaireCurrentKey(state)).toBe('opening-1')
    state = save(state)
    expect(questionnaireCurrentKey(state)).toBe('closing')
    state = save(state)
    expect(state.phase).toBe('complete')
    expect(state.coveredParts).toEqual(['description', 'opening', 'closing'])
    expect(questionnaireSavedCount(state)).toBe(10)
  })
})
