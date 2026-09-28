import { useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { GuidelineService } from '@/shared/api'
import { globalScope, type GuidelineKind, type GuidelineScope } from '../model/types'
import { invalidateGuidelineCandidates, invalidateGuidelines } from './guideline-cache'
import { guidelineErrorMessage } from './guideline-errors'
import { fromGuidelineKind, toScopePatch } from './guideline-queries'

/** The three write callers live with the entity rather than in the action slices because the
 *  revision capture needs the create one too, and a feature may not import a sibling feature.
 *  They are plain CRUD over the entity's own cache: no guideline write calls a model or enqueues a
 *  job ([I5]), and none of them touches a post, job or experiment cache — nothing references a
 *  guideline, so nothing else can go stale. */
export function useCreateGuidelineCall(ownerId: string, kind: GuidelineKind = 'post') {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.createGuideline, {
    onSuccess: () => invalidateGuidelines(queryClient, transport, ownerId, kind),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    /** `fromCandidateId` is set only when approving a candidate whose text was EDITED first, so
     *  the row can no longer be matched by text. The server marks it approved in the same
     *  transaction as the insert, which is also what keeps an on-the-spot 지침으로 저장 from
     *  reappearing as a candidate — that path matches by text and needs no id. */
    create: ({
      title = '',
      text,
      scope,
      fromCandidateId,
    }: {
      /** Optional (GUIDE-46); empty is none. */
      title?: string
      text: string
      scope: GuidelineScope
      fromCandidateId?: string
    }) =>
      mutation.mutateAsync({
        title: title.trim(),
        text: text.trim(),
        ...toScopePatch(scope),
        fromCandidateId,
        kind: fromGuidelineKind(kind),
      }),
  }
}

/** 무시. It marks the row rather than deleting it — the dismissed row is what keeps the same
 *  instruction from being recorded again — so nothing here is a delete and nothing is undoable
 *  beyond writing the guideline by hand. */
export function useDismissGuidelineCandidateCall(ownerId: string, kind: GuidelineKind = 'post') {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.dismissGuidelineCandidate, {
    onSuccess: () => invalidateGuidelineCandidates(queryClient, transport, ownerId, kind),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    dismiss: (id: string) => mutation.mutateAsync({ id }),
  }
}

/** Presence is the edit unit (GUIDE-6): a save carries only the parts it names, so an edit never
 *  rewrites a part another tab changed meanwhile. Sending every part every time would be a
 *  read-modify-write and would put back whatever the other tab changed; `save` takes the parts a
 *  form changed and sends them in one request, which the server applies in one transaction.
 *
 *  The scope goes as ONE patch because a scope is a kind plus a set: replacing them separately
 *  would leave a window where `global` still carries links. */
export function useUpdateGuidelineCall(
  ownerId: string,
  guidelineId: string,
  kind: GuidelineKind = 'post',
) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.updateGuideline, {
    onSuccess: () => invalidateGuidelines(queryClient, transport, ownerId, kind),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    save: (patch: { title?: string; text?: string; scope?: GuidelineScope }) =>
      mutation.mutateAsync({
        id: guidelineId,
        ...(patch.title !== undefined && { title: patch.title.trim() }),
        ...(patch.text !== undefined && { text: patch.text.trim() }),
        ...(patch.scope !== undefined && { scope: toScopePatch(patch.scope) }),
      }),
  }
}

export function useDeleteGuidelineCall(ownerId: string, kind: GuidelineKind = 'post') {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.deleteGuideline, {
    onSuccess: () => invalidateGuidelines(queryClient, transport, ownerId, kind),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    remove: (id: string) => mutation.mutateAsync({ id }),
  }
}

/** What a bulk run did: how many rows it moved, and which ones the server refused and why. */
export interface BulkReviewOutcome {
  moved: number
  failures: { id: string; message: string }[]
}

/** 전부 수락 / 전부 거절 (GUIDE-27). Both walk the rows the user is looking at and call the
 *  ordinary per-row procedure once each — there is no bulk endpoint, and there should not be one:
 *  the create is what owns the text bound, the account cap and the duplicate rule, and a batch
 *  that bypassed it would have to reimplement all three.
 *
 *  Sequential, never parallel: the dedupe read, the guideline check and the cap all run inside one
 *  transaction per create, so concurrent creates race the cap. A refusal is collected and the walk
 *  continues — one over-long candidate must not hold back the rest — and the caches are
 *  invalidated once at the end rather than once per row. */
export function useBulkReviewGuidelineCandidates(ownerId: string, kind: GuidelineKind = 'post') {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const [running, setRunning] = useState<'approve' | 'dismiss' | null>(null)

  const guidelineKind = kind
  const walk = async (
    action: 'approve' | 'dismiss',
    ids: readonly { id: string; text: string }[],
  ): Promise<BulkReviewOutcome> => {
    const client = createClient(GuidelineService, transport)
    const failures: BulkReviewOutcome['failures'] = []
    let moved = 0
    setRunning(action)
    try {
      for (const candidate of ids) {
        try {
          if (action === 'approve') {
            // 전역, because a scope is a decision per rule and the ones that need a narrower one
            // are exactly the ones worth opening 승인 for.
            await client.createGuideline({
              text: candidate.text.trim(),
              ...toScopePatch(globalScope()),
              fromCandidateId: candidate.id,
              kind: fromGuidelineKind(guidelineKind),
            })
          } else {
            await client.dismissGuidelineCandidate({ id: candidate.id })
          }
          moved += 1
        } catch (cause) {
          failures.push({ id: candidate.id, message: guidelineErrorMessage(cause) })
        }
      }
    } finally {
      setRunning(null)
      invalidateGuidelineCandidates(queryClient, transport, ownerId, guidelineKind)
      if (action === 'approve' && moved > 0)
        invalidateGuidelines(queryClient, transport, ownerId, guidelineKind)
    }
    return { moved, failures }
  }

  return {
    running,
    isPending: running !== null,
    approveAll: (candidates: readonly { id: string; text: string }[]) =>
      walk('approve', candidates),
    dismissAll: (candidates: readonly { id: string; text: string }[]) =>
      walk('dismiss', candidates),
  }
}
