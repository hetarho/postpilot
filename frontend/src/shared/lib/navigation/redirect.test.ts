import { describe, expect, it } from 'vitest'
import { isInAppPath } from './redirect'

describe('isInAppPath', () => {
  it('accepts in-app paths', () => {
    for (const value of ['/', '/posts', '/posts/5?tab=a#top', '/a%20b']) {
      expect(isInAppPath(value), value).toBe(true)
    }
  })

  it('rejects anything that resolves to another origin', () => {
    // Each of these is a way to leave the site while still starting with '/'.
    // `/\evil.com` is the one a hand-written "starts with a single slash" check misses:
    // the URL parser folds the backslash into a slash, making it scheme-relative.
    for (const value of ['//evil.example.com', '/\\evil.example.com', '/\\\\evil.example.com']) {
      expect(isInAppPath(value), value).toBe(false)
    }
  })

  it('rejects absolute URLs and non-http schemes', () => {
    for (const value of [
      'https://evil.example.com',
      'http://x',
      'javascript:alert(1)',
      'data:,x',
    ]) {
      expect(isInAppPath(value), value).toBe(false)
    }
  })

  it('rejects a missing or relative value', () => {
    for (const value of [undefined, '', 'posts', './x', '../x']) {
      expect(isInAppPath(value), String(value)).toBe(false)
    }
  })
})
