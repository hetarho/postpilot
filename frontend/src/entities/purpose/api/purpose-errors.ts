import { ConnectError } from '@connectrpc/connect'

/** The server's refusal, verbatim.
 *
 *  Its message already names the field and the limit in Korean, and it is the authority on
 *  both, so restating it here would give the user two slightly different rules to reconcile.
 *  Only a message-less failure (a transport error, an Internal the handler deliberately
 *  scrubbed) falls back to generic text.
 *
 *  It lives in the entity rather than in one of the three action slices because all three
 *  need it, and a feature may not import a sibling feature. */
export function purposeErrorMessage(error: unknown): string {
  if (!error) return ''
  const message = ConnectError.from(error).rawMessage.trim()
  return message || '용도를 저장하지 못했어요. 다시 시도해 주세요.'
}
