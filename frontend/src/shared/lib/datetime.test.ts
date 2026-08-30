import { describe, expect, it } from 'vitest'
import { formatRelativeTime } from './datetime'

const NOW = new Date('2026-08-28T12:00:00Z')

describe('formatRelativeTime', () => {
  it.each([
    ['12:00:00Z', '방금'],
    ['11:59:31Z', '방금'],
    ['11:57:00Z', '3분 전'],
    ['09:30:00Z', '2시간 전'],
  ])('renders %s as %s', (time, expected) => {
    expect(formatRelativeTime(`2026-08-28T${time}`, NOW)).toBe(expected)
  })

  it('counts in days up to a week', () => {
    expect(formatRelativeTime('2026-08-25T12:00:00Z', NOW)).toBe('3일 전')
  })

  it('falls back to the date once a week has passed', () => {
    expect(formatRelativeTime('2026-08-01T12:00:00Z', NOW)).toMatch(/2026/)
  })

  // A clock a little ahead of the server must not produce a negative count.
  it('treats a future timestamp as just now', () => {
    expect(formatRelativeTime('2026-08-28T12:00:20Z', NOW)).toBe('방금')
  })

  it.each([
    ['', 'empty'],
    ['not-a-date', 'malformed'],
  ])('renders nothing for a %s value (%s)', (value) => {
    expect(formatRelativeTime(value, NOW)).toBe('')
  })
})
