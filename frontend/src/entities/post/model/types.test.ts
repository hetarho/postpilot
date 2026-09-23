import i18next from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  POST_STATUSES,
  displayTitle,
  isPostStatus,
  isPublished,
  postStatusLabel,
  untitledTitle,
} from './types'

describe('displayTitle', () => {
  it('keeps a real title', () => {
    expect(displayTitle({ title: '제주 3일' })).toBe('제주 3일')
  })

  it.each([
    ['', 'empty'],
    ['   ', 'whitespace only'],
  ])('falls back for a %s title (%s)', (title) => {
    expect(displayTitle({ title })).toBe(untitledTitle())
  })
})

describe('postStatusLabel', () => {
  it.each([
    ['draft', '초안'],
    ['review', '검토'],
    ['finalized', '확정'],
    ['published', '발행됨'],
  ])('labels %s', (status, expected) => {
    expect(postStatusLabel(status)).toBe(expected)
  })

  it('labels a published post in English too', async () => {
    await i18next.changeLanguage('en')
    try {
      expect(postStatusLabel('published')).toBe('Published')
    } finally {
      await i18next.changeLanguage('ko')
    }
  })

  // A status a later plan adds must still render as something.
  it('passes an unknown status through', () => {
    expect(postStatusLabel('archived')).toBe('archived')
  })
})

describe('the status union', () => {
  it('lists every status in lifecycle order', () => {
    expect(POST_STATUSES).toEqual(['draft', 'review', 'finalized', 'published'])
  })

  it('knows exactly the four', () => {
    for (const status of POST_STATUSES) expect(isPostStatus(status)).toBe(true)
    for (const other of ['', 'archived', 'Published', undefined, 3]) {
      expect(isPostStatus(other)).toBe(false)
    }
  })

  // The one predicate every lock reads (POST-86).
  it('calls only a published post published', () => {
    expect(POST_STATUSES.filter((status) => isPublished({ status }))).toEqual(['published'])
  })
})
