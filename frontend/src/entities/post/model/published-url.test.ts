/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { POST_PUBLISHED_URL_MAX_CHARS } from '../config'
import { parseNaverBlogUrl } from './published-url'

interface FixtureCase {
  name: string
  input: string
  /** The stored form, or null for a refusal. */
  stored: string | null
}

// The same file the server's parser runs (POST-77), so the two cannot disagree: a new rule is a
// new case there, and both sides see it.
const fixture = JSON.parse(
  readFileSync(
    resolve(
      import.meta.dirname,
      '../../../../../backend/internal/post/testdata/published_url/cases.json',
    ),
    'utf8',
  ),
)
const prefix = 'https://blog.naver.com/alice/'
const atLimit = prefix + 'a'.repeat(POST_PUBLISHED_URL_MAX_CHARS - prefix.length)
const cases: FixtureCase[] = [
  ...fixture.cases,
  // Built from the constant, as the Go harness builds them from PublishedURLMaxChars.
  { name: 'an address at the length limit', input: atLimit, stored: atLimit },
  { name: 'an address over the length limit', input: `${atLimit}a`, stored: null },
]

describe('parseNaverBlogUrl', () => {
  it('reads the shared fixture', () => {
    expect(fixture.cases.length).toBeGreaterThan(0)
  })

  it.each(cases)('$name', ({ input, stored }) => {
    expect(parseNaverBlogUrl(input)).toBe(stored ?? undefined)
  })

  // Code points, not UTF-16 units, as the server counts runes: a character outside the BMP is
  // two units and one character.
  it('counts the limit in code points', () => {
    const astral = prefix + '𠀀'.repeat(POST_PUBLISHED_URL_MAX_CHARS - prefix.length)
    expect(astral.length).toBeGreaterThan(POST_PUBLISHED_URL_MAX_CHARS)
    expect(parseNaverBlogUrl(astral)).toBe(astral)
    expect(parseNaverBlogUrl(`${astral}𠀀`)).toBeUndefined()
  })
})
