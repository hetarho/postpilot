import { describe, expect, it } from 'vitest'

import { ContentLanguage as ProtoContentLanguage } from './gen/postpilot/v1/language_pb'
import {
  contentLanguageFromProto,
  contentLanguageToProto,
  requireContentLanguage,
} from './language'

describe('content language transport mapping', () => {
  it('covers every generated enum value without treating absence as a language', () => {
    const generated = Object.values(ProtoContentLanguage).filter(
      (value): value is ProtoContentLanguage => typeof value === 'number',
    )
    expect(generated).toEqual([
      ProtoContentLanguage.UNSPECIFIED,
      ProtoContentLanguage.KOREAN,
      ProtoContentLanguage.ENGLISH,
    ])
    expect(generated.map(contentLanguageFromProto)).toEqual([undefined, 'ko', 'en'])
  })

  it.each([
    ['ko', ProtoContentLanguage.KOREAN],
    ['en', ProtoContentLanguage.ENGLISH],
  ] as const)('round-trips %s', (domain, proto) => {
    expect(contentLanguageToProto(domain)).toBe(proto)
    expect(requireContentLanguage(proto)).toBe(domain)
  })

  it('keeps unspecified as absence for optional projections', () => {
    expect(contentLanguageFromProto(ProtoContentLanguage.UNSPECIFIED)).toBeUndefined()
  })

  it('rejects unspecified and unknown enum values at required boundaries', () => {
    expect(() => requireContentLanguage(ProtoContentLanguage.UNSPECIFIED)).toThrow(
      'unsupported content language enum',
    )
    expect(() => requireContentLanguage(999 as ProtoContentLanguage)).toThrow(
      'unsupported content language enum',
    )
  })
})
