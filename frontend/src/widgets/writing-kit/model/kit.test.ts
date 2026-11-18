import { describe, expect, it } from 'vitest'
import {
  aiElement,
  failedElement,
  guidelineElement,
  kitSummary,
  listElement,
  nextKitStep,
  voiceElement,
} from './kit'

const voice = (over: Partial<Parameters<typeof voiceElement>[0][number]> = {}) => ({
  name: '내 말투',
  made: false,
  readinessPercent: 0,
  isDefault: false,
  ...over,
})

describe('the writing kit reads readiness from the account’s own data', () => {
  it('reads 말투 from the voice a post is written in, made or by its own meter', () => {
    expect(voiceElement([], false)).toMatchObject({ state: 'empty', fill: 0 })
    expect(voiceElement([voice({ readinessPercent: 47 })], false)).toMatchObject({
      state: 'partial',
      fill: 0.47,
      fact: { kind: 'voice', percent: 47, made: false },
    })
    // The default voice is the writing voice even when another one is made.
    const element = voiceElement(
      [
        voice({ name: '다른', made: true }),
        voice({ name: '기본', isDefault: true, readinessPercent: 20 }),
      ],
      false,
    )
    expect(element).toMatchObject({ state: 'partial', fact: { name: '기본' } })
    expect(voiceElement([voice({ made: true })], false)).toMatchObject({ state: 'ready', fill: 1 })
    expect(voiceElement([], true).state).toBe('loading')
  })

  it('counts 템플릿 and 기억 ready from the first one and names the most used template', () => {
    expect(listElement('template', [], false).state).toBe('empty')
    expect(
      listElement(
        'template',
        [
          { name: '일기', postCount: 1 },
          { name: '맛집', postCount: 4 },
        ],
        false,
      ),
    ).toMatchObject({ state: 'ready', fact: { kind: 'count', count: 2, lead: '맛집' } })
    expect(listElement('memory', [{}, {}, {}], false)).toMatchObject({
      state: 'ready',
      fact: { kind: 'count', count: 3 },
    })
  })

  it('calls 지침 ready only with a rule of the owner’s own, and says when defaults alone apply', () => {
    expect(guidelineElement(0, 3, false)).toMatchObject({
      state: 'empty',
      fact: { kind: 'defaultsOnly', count: 3 },
    })
    expect(guidelineElement(2, 3, false)).toMatchObject({ state: 'ready', fact: { count: 2 } })
  })

  it('calls the AI ready only while the write stage has a usable model', () => {
    expect(aiElement('추천 AI', false, false)).toMatchObject({
      state: 'ready',
      fact: { kind: 'model', label: '추천 AI' },
    })
    expect(aiElement(null, true, false)).toMatchObject({
      state: 'empty',
      fact: { kind: 'unavailable' },
    })
  })

  it('points at the AI first, then the voice, and is complete only when all five are ready', () => {
    const elements = [
      voiceElement([voice({ readinessPercent: 50 })], false),
      listElement('template', [{ name: 'a' }], false),
      guidelineElement(0, 0, false),
      aiElement(null, false, false),
      listElement('memory', [], false),
    ]
    expect(nextKitStep(elements)?.key).toBe('ai')
    expect(nextKitStep(elements.filter((element) => element.key !== 'ai'))?.key).toBe('voice')
    expect(kitSummary(elements)).toMatchObject({ ready: 1, total: 5, complete: false })
    const ready = [
      voiceElement([voice({ made: true })], false),
      listElement('template', [{ name: 'a' }], false),
      guidelineElement(1, 0, false),
      aiElement('AI', false, false),
      listElement('memory', [{}], false),
    ]
    expect(nextKitStep(ready)).toBeUndefined()
    expect(kitSummary(ready)).toMatchObject({ ready: 5, complete: true, fill: 1 })
  })

  it('never calls a piece that could not be read missing, nor points at it next', () => {
    const elements = [
      voiceElement([voice({ made: true })], false),
      failedElement('template'),
      guidelineElement(1, 0, false),
      aiElement('AI', false, false),
      listElement('memory', [], false),
    ]
    expect(elements[1]).toMatchObject({ state: 'failed', fill: 0, fact: { kind: 'failed' } })
    expect(nextKitStep(elements)?.key).toBe('memory')
    expect(kitSummary(elements)).toMatchObject({ failed: true, ready: 3, complete: false })
  })
})
