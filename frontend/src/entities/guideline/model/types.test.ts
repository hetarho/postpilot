import { describe, expect, it } from 'vitest'
import { canSaveGuideline, globalScope, type GuidelineScope } from './types'

// GUIDE-5: each kind carries its own set and never another kind's, and a narrowed kind carries at
// least one. The client check only stops an obviously bad save; the server stays authoritative.
describe('the guideline scope shapes', () => {
  it.each<[string, GuidelineScope]>([
    ['global', globalScope()],
    ['templates', { kind: 'templates', templateIds: ['t'], fields: [] }],
    ['fields', { kind: 'fields', templateIds: [], fields: ['cafe'] }],
  ])('accepts a valid %s scope', (_label, scope) => {
    expect(canSaveGuideline('가격을 지어내지 않기', scope)).toBe(true)
  })

  it.each<[string, GuidelineScope]>([
    ['global with a template', { kind: 'global', templateIds: ['t'], fields: [] }],
    ['global with a 분야', { kind: 'global', templateIds: [], fields: ['cafe'] }],
    ['templates with a 분야', { kind: 'templates', templateIds: ['t'], fields: ['cafe'] }],
    ['templates with none', { kind: 'templates', templateIds: [], fields: [] }],
    ['fields with a template', { kind: 'fields', templateIds: ['t'], fields: ['cafe'] }],
    ['fields with none', { kind: 'fields', templateIds: [], fields: [] }],
  ])('refuses %s', (_label, scope) => {
    expect(canSaveGuideline('가격을 지어내지 않기', scope)).toBe(false)
  })

  it('still refuses an empty text whatever the scope', () => {
    expect(canSaveGuideline('   ', { kind: 'fields', templateIds: [], fields: ['cafe'] })).toBe(
      false,
    )
  })

  it('starts every new scope as 전역 with neither set', () => {
    expect(globalScope()).toEqual({ kind: 'global', templateIds: [], fields: [] })
  })
})
