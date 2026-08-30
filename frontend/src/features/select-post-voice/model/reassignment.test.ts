import { Code, ConnectError } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { reassignmentBlocker, reassignmentFailureMessage } from './reassignment'

describe('reassignmentBlocker', () => {
  it('allows an idle post', () => {
    expect(reassignmentBlocker({ activeJob: undefined, pendingExperimentId: '' })).toBe('')
    expect(reassignmentBlocker({ activeJob: { status: 'done' }, pendingExperimentId: '' })).toBe('')
  })

  it('blocks while a job runs or an A/B result waits', () => {
    expect(reassignmentBlocker({ activeJob: { status: 'running' }, pendingExperimentId: '' })).toMatch(
      /AI 작업/,
    )
    expect(reassignmentBlocker({ activeJob: undefined, pendingExperimentId: 'exp' })).toMatch(/A\/B/)
  })
})

describe('reassignmentFailureMessage', () => {
  it('names the precondition and falls back for anything else', () => {
    expect(reassignmentFailureMessage(new ConnectError('busy', Code.FailedPrecondition))).toMatch(
      /지금은 말투를 바꿀 수 없어요/,
    )
    expect(reassignmentFailureMessage(new ConnectError('gone', Code.NotFound))).toMatch(/찾을 수 없어요/)
    expect(reassignmentFailureMessage(new Error('boom'))).toBe('말투를 바꾸지 못했어요. 다시 시도해 주세요.')
  })
})
