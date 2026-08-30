import { Code, ConnectError } from '@connectrpc/connect'

/** What the editor says beside the select while a job is running.
 *
 *  Not a blocker, unlike a voice reassignment: a purpose is never learned from, so changing
 *  it is allowed in every status and while a job is in flight (spec/policy/purposes.md). The
 *  running job keeps the brief frozen at its enqueue, and the note is there so nobody expects
 *  the change to reach work already queued. */
export const RUNNING_JOB_NOTE =
  '진행 중인 AI 작업은 시작할 때의 용도로 끝나요. 바꾼 용도는 다음 생성부터 적용됩니다.'

/** The server's refusal, in the user's words. */
export function assignmentFailureMessage(cause: unknown): string {
  switch (ConnectError.from(cause).code) {
    case Code.NotFound:
      return '고른 용도를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요.'
    default:
      return '용도를 바꾸지 못했어요. 다시 시도해 주세요.'
  }
}
