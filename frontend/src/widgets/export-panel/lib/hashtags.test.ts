import { describe, expect, it } from 'vitest'
import { toHashtags } from './hashtags'

describe('toHashtags', () => {
  it('prefixes each tag once and separates with a single space, with no trailing separator', () => {
    expect(toHashtags(['tag1', 'tag2', 'tag3'])).toBe('#tag1 #tag2 #tag3')
  })

  it('does not double-prefix a tag the model already wrote with a #', () => {
    expect(toHashtags(['#제주', '산책', '##여행'])).toBe('#제주 #산책 #여행')
  })

  it('trims and drops tags that are empty once trimmed', () => {
    expect(toHashtags(['  제주  ', '', '   ', '#  ', '산책'])).toBe('#제주 #산책')
  })

  it('is empty for an empty list, which is what suppresses the field', () => {
    expect(toHashtags([])).toBe('')
    expect(toHashtags(['', '  '])).toBe('')
  })

  it('keeps a tag that contains a space as one tag', () => {
    expect(toHashtags(['비 온 뒤'])).toBe('#비 온 뒤')
  })
})
