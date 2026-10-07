import { describe, expect, it } from 'vitest'
import {
  OriginFieldKind,
  OriginReviewState,
  SemanticOriginCategory,
} from './gen/postpilot/v1/semantic_origin_pb'
import {
  originFieldKindFromProto,
  originFieldKindNames,
  originReviewStateFromProto,
  originReviewStateNames,
  semanticOriginCategoryFromProto,
  semanticOriginCategoryNames,
} from './semantic-origin'

describe('semantic origin transport vocabulary', () => {
  it.each([
    [SemanticOriginCategory, semanticOriginCategoryNames, semanticOriginCategoryFromProto],
    [OriginReviewState, originReviewStateNames, originReviewStateFromProto],
    [OriginFieldKind, originFieldKindNames, originFieldKindFromProto],
  ] as const)(
    'explicitly maps each generated enum including absence',
    (generated, names, mapper) => {
      const mapValue = mapper as (value: number) => string | undefined
      const values = Object.values(generated).filter((value) => typeof value === 'number')
      expect(Object.keys(names).map(Number)).toEqual(values)
      expect(mapValue(0)).toBeUndefined()
      expect(mapValue(999)).toBeUndefined()
      for (const value of values.filter((value) => value !== 0))
        expect(mapValue(value)).toBeTypeOf('string')
    },
  )
})
