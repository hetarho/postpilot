import { Code, ConnectError } from '@connectrpc/connect'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'

/** Mirrors the server's reassignment gate (spec/policy/posts.md) so the picker can say why it is
 *  disabled instead of letting the request fail: a job or an undecided A/B result could still
 *  establish a baseline for the old voice. Returns '' when reassignment is allowed. */
export function reassignmentBlocker(post: {
  activeJob: Pick<GenerationJob, 'status'> | undefined
  pendingExperimentId: string
}): string {
  if (post.activeJob && !isTerminal(post.activeJob))
    return 'AI 작업이 끝나면 말투를 바꿀 수 있어요.'
  if (post.pendingExperimentId) return '대기 중인 A/B 결과를 먼저 확인하면 말투를 바꿀 수 있어요.'
  return ''
}

/** The server's refusal, in the user's words. FailedPrecondition covers both a post that became
 *  busy and a voice deleted since the list was read; the message names both. */
export function reassignmentFailureMessage(cause: unknown): string {
  switch (ConnectError.from(cause).code) {
    case Code.FailedPrecondition:
      return '지금은 말투를 바꿀 수 없어요. 진행 중인 AI 작업이 끝났는지, 고른 말투가 아직 있는지 확인해 주세요.'
    case Code.NotFound:
      return '고른 말투를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요.'
    default:
      return '말투를 바꾸지 못했어요. 다시 시도해 주세요.'
  }
}
