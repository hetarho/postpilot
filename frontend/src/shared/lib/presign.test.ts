import { describe, expect, it } from 'vitest'
import { presignExpired } from './presign'

/** A view URL the way the API mints it: SigV4 states the lifetime in the query, which is what
 *  tells an expired read apart from a bucket that allows this origin no `GET`. */
function presigned(signedAt: Date, lifetimeSeconds: number) {
  const stamp = signedAt
    .toISOString()
    .replace(/[-:]/g, '')
    .replace(/\.\d{3}/, '')
  return `https://bucket.test/IMG_1.jpg?X-Amz-Date=${stamp}&X-Amz-Expires=${lifetimeSeconds}`
}

describe('presignExpired', () => {
  it('answers from the URL and the clock, on both sides of the lifetime', () => {
    const signedAt = new Date('2026-09-04T01:02:03Z')
    const url = presigned(signedAt, 600)
    expect(presignExpired(url, signedAt.getTime() + 599_000)).toBe(false)
    expect(presignExpired(url, signedAt.getTime() + 601_000)).toBe(true)
  })

  // The whole point of the helper: an expired read and a bucket that allows this origin no `GET`
  // are the same event in the browser, and they lead to opposite advice. A URL that states no
  // lifetime cannot be claimed to have expired, so the advice falls to the other side.
  it('never claims expiry for a URL that carries no SigV4 lifetime', () => {
    expect(presignExpired('https://bucket.test/IMG_1.jpg', Date.now())).toBe(false)
    expect(presignExpired('blob:local', Date.now())).toBe(false)
    expect(presignExpired('not a url at all', Date.now())).toBe(false)
    expect(presignExpired('https://bucket.test/IMG_1.jpg?X-Amz-Expires=600', Date.now())).toBe(
      false,
    )
    expect(
      presignExpired(
        `https://bucket.test/IMG_1.jpg?X-Amz-Date=20260904T010203Z&X-Amz-Expires=0`,
        Date.now(),
      ),
    ).toBe(false)
  })

  // `presigned()` above builds the shape this parser reads, so on its own it could agree with a
  // parser that is wrong about the real thing. This URL is the shape aws-sdk-go-v2's
  // `PresignGetObject` actually emits (`backend/internal/storage/r2.go`), pinned as a literal —
  // including `X-Amz-Date`'s ISO 8601 *basic* form, which `Date.parse` need not accept.
  it('reads the lifetime out of a real presigned view URL', () => {
    const real =
      'https://acct.r2.cloudflarestorage.com/postpilot-prod/u/1/IMG_1.jpg' +
      '?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=key%2F20260904%2Fauto%2Fs3%2Faws4_request' +
      '&X-Amz-Date=20260904T010203Z&X-Amz-Expires=600&X-Amz-SignedHeaders=host&X-Amz-Signature=abc'
    expect(presignExpired(real, Date.parse('2026-09-04T01:05:00Z'))).toBe(false)
    expect(presignExpired(real, Date.parse('2026-09-04T02:00:00Z'))).toBe(true)
  })
})
