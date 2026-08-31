import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { GuidelineService } from '@/shared/api'
import type { GuidelineScope } from '../model/types'
import { invalidateGuidelines } from './guideline-cache'
import { guidelineErrorMessage } from './guideline-errors'
import { toScopePatch } from './guideline-queries'

/** The three write callers live with the entity rather than in the action slices because the
 *  revision capture needs the create one too, and a feature may not import a sibling feature.
 *  They are plain CRUD over the entity's own cache: no guideline write calls a model or enqueues a
 *  job ([I5]), and none of them touches a post, job or experiment cache — nothing references a
 *  guideline, so nothing else can go stale. */
export function useCreateGuidelineCall(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.createGuideline, {
    onSuccess: () => invalidateGuidelines(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    create: (text: string, scope: GuidelineScope) =>
      mutation.mutateAsync({ text: text.trim(), ...toScopePatch(scope) }),
  }
}

/** Presence is the edit unit (spec/policy/guidelines.md): the text saver sends no scope and the
 *  scope saver sends no text, so the two edited from two tabs cannot overwrite each other. Sending
 *  both every time would be a read-modify-write and would put back whatever the other tab changed.
 *
 *  The scope goes as ONE patch because a scope is a kind plus a set: replacing them separately
 *  would leave a window where `global` still carries links. */
export function useUpdateGuidelineCall(ownerId: string, guidelineId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.updateGuideline, {
    onSuccess: () => invalidateGuidelines(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    saveText: (text: string) => mutation.mutateAsync({ id: guidelineId, text: text.trim() }),
    saveScope: (scope: GuidelineScope) =>
      mutation.mutateAsync({ id: guidelineId, scope: toScopePatch(scope) }),
  }
}

export function useDeleteGuidelineCall(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GuidelineService.method.deleteGuideline, {
    onSuccess: () => invalidateGuidelines(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    remove: (id: string) => mutation.mutateAsync({ id }),
  }
}
