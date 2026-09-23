import { describe, expect, it } from 'vitest'
import { ProtoBlogField } from '@/shared/api'
import { BLOG_FIELD_IDS, NO_BLOG_FIELD } from '../model/blog-field'
import { blogFieldFromProto, blogFieldToProto } from './blog-field-mappers'

describe('the 분야 wire mapping', () => {
  // One list on both sides. A 분야 added to the contract without an id here would read as
  // unknown on every post that carries it, which is exactly what this catches (ARCH-3).
  it('maps every value the wire names, both ways', () => {
    const wireValues = Object.values(ProtoBlogField).filter(
      (value): value is ProtoBlogField => typeof value === 'number',
    )
    expect(wireValues).toHaveLength(BLOG_FIELD_IDS.length + 1)
    for (const wire of wireValues) {
      const choice = blogFieldFromProto(wire)
      expect(choice, `wire value ${wire} has no choice`).toBeDefined()
      expect(blogFieldToProto(choice!)).toBe(wire)
    }
  })

  it('reads UNSPECIFIED as 없음 and sends 없음 as UNSPECIFIED', () => {
    expect(blogFieldFromProto(ProtoBlogField.UNSPECIFIED)).toBe(NO_BLOG_FIELD)
    expect(blogFieldToProto(NO_BLOG_FIELD)).toBe(ProtoBlogField.UNSPECIFIED)
  })

  it('gives each id its own wire value, in catalogue order', () => {
    expect(BLOG_FIELD_IDS.map(blogFieldToProto)).toEqual([
      ProtoBlogField.RESTAURANT,
      ProtoBlogField.CAFE,
      ProtoBlogField.DOMESTIC_TRAVEL,
      ProtoBlogField.FASHION_BEAUTY,
      ProtoBlogField.PRODUCT_REVIEW,
      ProtoBlogField.PARENTING_MARRIAGE,
      ProtoBlogField.PETS,
      ProtoBlogField.INTERIOR_DIY,
      ProtoBlogField.DAILY_LIFE,
    ])
  })

  it('drops a wire value this build does not know rather than inventing one', () => {
    expect(blogFieldFromProto(9_999 as ProtoBlogField)).toBeUndefined()
  })
})
