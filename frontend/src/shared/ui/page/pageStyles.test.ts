import { describe, expect, it } from 'vitest'
import { pageStyles } from './pageStyles'

describe('pageStyles', () => {
  it('caps nothing on a phone: every width and gutter it sets is prefixed or full-bleed', () => {
    // design-language §1.5's test, mechanised: strip every breakpoint-prefixed class and what is
    // left must still be a finished screen — a full-width column with the phone's own gutter.
    for (const width of ['prose', 'wide', 'board'] as const) {
      const unprefixed = pageStyles({ width })
        .split(' ')
        .filter((token) => !token.includes(':'))
      expect(unprefixed, width).toContain('w-full')
      expect(unprefixed, width).toContain('px-4')
      // A cap that is NOT behind a breakpoint would squeeze the phone, which has less room than
      // the smallest of them.
      expect(
        unprefixed.filter((token) => token.startsWith('max-w-')),
        width,
      ).toEqual([])
    }
  })

  it('gives every kind of screen more room on the desk than on the laptop', () => {
    // The bug this primitive exists for: a 1440px desk showing a phone-width column with ~380px
    // of empty gutter on either side. Each kind must widen at `lg:`, and never past its neighbour.
    const desk = (width: 'prose' | 'wide' | 'board') =>
      pageStyles({ width })
        .split(' ')
        .find((token) => token.startsWith('lg:max-w-'))
    expect(desk('prose')).toBe('lg:max-w-3xl')
    expect(desk('wide')).toBe('lg:max-w-5xl')
    expect(desk('board')).toBe('lg:max-w-7xl')
  })

  it('drops the gutter for a screen whose rows run edge to edge', () => {
    const bare = pageStyles({ gutters: false }).split(' ')
    expect(bare).not.toContain('px-4')
    expect(bare).not.toContain('sm:px-6')
    expect(bare).not.toContain('lg:px-8')
    expect(bare).toContain('w-full')
  })

  it('resolves a caller override toward the call site rather than doubling it', () => {
    // twMerge, not concatenation: a page that wants a tighter rhythm says `py-10` and gets one
    // padding, not two competing ones in source order.
    const tokens = pageStyles({ className: 'flex flex-1 flex-col py-10' }).split(' ')
    expect(tokens).toContain('py-10')
    expect(tokens).not.toContain('py-6')
    expect(tokens).toContain('flex-1')
  })
})
