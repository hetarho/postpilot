import { describe, expect, it } from 'vitest'
import { UNTITLED_TITLE, displayTitle, postStatusLabel } from './types'

describe('displayTitle', () => {
  it('keeps a real title', () => {
    expect(displayTitle({ title: '제주 3일' })).toBe('제주 3일')
  })

  it.each([
    ['', 'empty'],
    ['   ', 'whitespace only'],
  ])('falls back for a %s title (%s)', (title) => {
    expect(displayTitle({ title })).toBe(UNTITLED_TITLE)
  })
})

describe('postStatusLabel', () => {
  it.each([
    ['draft', '초안'],
    ['review', '검토'],
  ])('labels %s', (status, expected) => {
    expect(postStatusLabel(status)).toBe(expected)
  })

  // A status a later plan adds must still render as something.
  it('passes an unknown status through', () => {
    expect(postStatusLabel('archived')).toBe('archived')
  })
})
