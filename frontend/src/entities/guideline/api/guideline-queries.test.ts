import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  GuidelinePresetSchema,
  GuidelineSchema,
  ProtoBlogField,
  ProtoGuidelineScope,
} from '@/shared/api'
import {
  fromScopeKind,
  toGuideline,
  toGuidelinePreset,
  toScopeKind,
  toScopePatch,
} from './guideline-queries'

const wireScopes = Object.values(ProtoGuidelineScope).filter(
  (value): value is ProtoGuidelineScope =>
    typeof value === 'number' && value !== ProtoGuidelineScope.UNSPECIFIED,
)

// ARCH-3: the mirror is pinned against the generated enum, so a scope the contract gains without a
// kind here fails this file rather than being listed under a guessed one.
describe('the guideline scope mirror', () => {
  it('names every generated scope and round-trips it', () => {
    expect(wireScopes.map(toScopeKind)).toEqual(['global', 'templates', 'fields'])
    for (const wire of wireScopes) expect(fromScopeKind(toScopeKind(wire)!)).toBe(wire)
  })

  it.each([
    ['UNSPECIFIED', ProtoGuidelineScope.UNSPECIFIED],
    ['an unknown number', 99 as ProtoGuidelineScope],
  ])('reads %s as no scope, and a guideline carrying it cannot be read', (_label, scope) => {
    expect(toScopeKind(scope)).toBeUndefined()
    // GUIDE-14: a guessed scope would misstate which posts the rule reaches.
    expect(() => toGuideline(create(GuidelineSchema, { id: 'g', text: '규칙', scope }))).toThrow(
      `unsupported guideline scope enum: ${scope}`,
    )
  })
})

describe('a guideline on the wire', () => {
  it('reads a 분야 guideline’s set', () => {
    const guideline = toGuideline(
      create(GuidelineSchema, {
        id: 'g',
        text: '가격을 지어내지 않기',
        scope: ProtoGuidelineScope.FIELDS,
        fields: [ProtoBlogField.CAFE, ProtoBlogField.RESTAURANT],
      }),
    )
    expect(guideline.scope).toBe('fields')
    expect(guideline.fields).toEqual(['cafe', 'restaurant'])
    expect(guideline.templates).toEqual([])
  })

  // ARCH-3: dropping an entry would let the next whole-set save erase it on the server, so a 분야
  // this build cannot name fails the read — and so does 없음, which the server never puts in a set.
  it.each([
    ['a number this build does not know', 9_999 as ProtoBlogField],
    ['UNSPECIFIED', ProtoBlogField.UNSPECIFIED],
  ])('refuses a set with a 분야 this build does not know: %s', (_name, field) => {
    expect(() =>
      toGuideline(
        create(GuidelineSchema, {
          id: 'g',
          text: '가격을 지어내지 않기',
          scope: ProtoGuidelineScope.FIELDS,
          fields: [ProtoBlogField.CAFE, field],
        }),
      ),
    ).toThrow()
  })

  it('reads the other scopes with no 분야', () => {
    const global = toGuideline(create(GuidelineSchema, { scope: ProtoGuidelineScope.GLOBAL }))
    expect(global).toMatchObject({ scope: 'global', fields: [] })
  })

  it('sends a whole scope, 분야 included, as one patch', () => {
    expect(
      toScopePatch({ kind: 'fields', templateIds: [], fields: ['restaurant', 'cafe'] }),
    ).toEqual({
      scope: ProtoGuidelineScope.FIELDS,
      templateIds: [],
      fields: [ProtoBlogField.RESTAURANT, ProtoBlogField.CAFE],
    })
    expect(toScopePatch({ kind: 'templates', templateIds: ['t'], fields: [] })).toEqual({
      scope: ProtoGuidelineScope.TEMPLATES,
      templateIds: ['t'],
      fields: [],
    })
  })
})

// GUIDE-29: the preset rides every list read, and its absence is a malformed read, not an empty one.
describe('the guideline preset on the wire', () => {
  it('maps the text, the switch and the 분야', () => {
    expect(
      toGuidelinePreset(
        create(GuidelinePresetSchema, {
          text: '[분야 상위 글 문구]\n원문에 없는 내용은 쓰지 않는다.',
          enabled: true,
          fields: [ProtoBlogField.RESTAURANT],
        }),
      ),
    ).toEqual({
      text: '[분야 상위 글 문구]\n원문에 없는 내용은 쓰지 않는다.',
      enabled: true,
      fields: ['restaurant'],
    })
  })

  it.each([
    ['a number this build does not know', 9_999 as ProtoBlogField],
    ['UNSPECIFIED', ProtoBlogField.UNSPECIFIED],
  ])('refuses a set with a 분야 this build does not know: %s', (_name, field) => {
    expect(() =>
      toGuidelinePreset(
        create(GuidelinePresetSchema, {
          text: '',
          enabled: true,
          fields: [ProtoBlogField.RESTAURANT, field],
        }),
      ),
    ).toThrow()
  })

  it('refuses a list read that carries no preset', () => {
    expect(() => toGuidelinePreset(undefined)).toThrow('guideline list carries no preset')
  })
})
