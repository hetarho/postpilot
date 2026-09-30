import { describe, expect, it } from 'vitest'
import { PostCreditsBasis } from './gen/postpilot/v1/provider_pb'
import { postCreditsBasisName } from './post-credits'

describe('the per-post credit basis crossing the wire', () => {
  // ARCH-3: every wire value but UNSPECIFIED has its own client name, so a value added to the
  // enum without one fails here instead of reaching a screen as nothing.
  it('names every declared basis and only those', () => {
    const values = Object.values(PostCreditsBasis).filter(
      (value): value is PostCreditsBasis => typeof value === 'number',
    )
    const named = values.filter((value) => value !== PostCreditsBasis.UNSPECIFIED)
    expect(new Set(named.map(postCreditsBasisName)).size).toBe(named.length)
    for (const value of named) expect(postCreditsBasisName(value)).toBeDefined()
    expect(postCreditsBasisName(PostCreditsBasis.UNSPECIFIED)).toBeUndefined()
  })
})
