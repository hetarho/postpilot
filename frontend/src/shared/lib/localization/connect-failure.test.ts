import { ConnectError, Code } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from './failure'
import { connectFailureMessage } from './connect-failure'

describe('connectFailureMessage', () => {
  it('is empty without an error', () => {
    expect(connectFailureMessage(null)).toBe('')
    expect(connectFailureMessage(undefined)).toBe('')
  })

  it('equals the composed form for a ConnectError', () => {
    const error = new ConnectError('boom', Code.Unavailable)
    expect(connectFailureMessage(error)).toBe(formatAppFailure(appFailureFromConnect(error)))
  })
})
