import { Code } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { connectAppError } from '@/test/app-error'
import { reassignmentBlocker, reassignmentFailureMessage } from './reassignment'

describe('reassignmentBlocker', () => {
  it('allows an idle post', () => {
    expect(reassignmentBlocker({ activeJob: undefined, pendingExperimentId: '' })).toBe('')
    expect(reassignmentBlocker({ activeJob: { status: 'done' }, pendingExperimentId: '' })).toBe('')
  })

  it.each(['', 'queued-test', 'frozen-test', 'retained-legacy-result'])(
    'allows future reassignment independently of %s',
    (pendingExperimentId) => {
      for (const status of ['done', 'failed', 'cancelled'] as const)
        expect(reassignmentBlocker({ activeJob: { status }, pendingExperimentId })).toBe('')
      expect(reassignmentBlocker({ activeJob: undefined, pendingExperimentId })).toBe('')
    },
  )

  it('blocks ordinary work while a job runs', () => {
    expect(
      reassignmentBlocker({ activeJob: { status: 'running' }, pendingExperimentId: '' }),
    ).toMatch(/AI 작업/)
    expect(reassignmentBlocker({ activeJob: undefined, pendingExperimentId: 'exp' })).toBe('')
  })
})

describe('reassignmentFailureMessage', () => {
  it('uses stable reasons and falls back without exposing raw prose', () => {
    expect(reassignmentFailureMessage(connectAppError('POST_BUSY', Code.FailedPrecondition))).toBe(
      '이 글에서 다른 작업이 진행 중이에요.',
    )
    expect(reassignmentFailureMessage(connectAppError('VOICE_NOT_FOUND', Code.NotFound))).toBe(
      '고른 말투를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요. 말투를 찾을 수 없어요.',
    )
    expect(reassignmentFailureMessage(new Error('boom'))).toBe(
      '요청을 마치지 못했어요. 다시 시도해 주세요.',
    )
  })
})
