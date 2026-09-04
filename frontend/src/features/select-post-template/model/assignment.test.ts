import { Code } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { connectAppError } from '@/test/app-error'
import { assignmentFailureMessage } from './assignment'

describe('assignmentFailureMessage', () => {
  it('translates a stable template refusal and hides backend prose', () => {
    expect(assignmentFailureMessage(connectAppError('TEMPLATE_NOT_FOUND', Code.NotFound))).toBe(
      '고른 템플릿을 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요. 템플릿을 찾을 수 없어요.',
    )
  })

  it('uses the safe fallback for an unstructured exception', () => {
    expect(assignmentFailureMessage(new Error('private detail'))).toBe(
      '요청을 마치지 못했어요. 다시 시도해 주세요.',
    )
  })
})
