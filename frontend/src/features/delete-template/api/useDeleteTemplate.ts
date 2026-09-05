import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidateGuidelines } from '@/entities/guideline'
import { postDetailQueriesKey, listPostsQueryKey } from '@/entities/post'
import { invalidateTemplates, templateErrorMessage } from '@/entities/template'
import { TemplateService } from '@/shared/api'

/** Deleting a template detaches it from every post that named it and cascades its guideline scope
 *  links, in the same transaction (spec/legacy/policy/templates.md). Those posts are now cached with an
 *  assignment the server no longer holds, and the guideline list's chips are cached from the name
 *  that is gone, so both are invalidated alongside the directory. */
export function useDeleteTemplate(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(TemplateService.method.deleteTemplate, {
    onSuccess: (data) => {
      invalidateTemplates(queryClient, transport, ownerId)
      // The delete cascaded this template's guideline scope links, so a guideline scoped to it now
      // shows different chips — or 적용 대상 없음, if it named no other template. Always, not only
      // when posts were detached: a guideline can be scoped to a template no post uses.
      invalidateGuidelines(queryClient, transport, ownerId)
      // Only when the delete actually detached something: an unreferenced template leaves
      // every post's cached badge correct, and refetching them all would be pure noise.
      if (data.detachedPosts > 0) {
        void queryClient.invalidateQueries({ queryKey: postDetailQueriesKey(transport) })
        void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
      }
    },
  })
  return {
    ...mutation,
    errorMessage: templateErrorMessage(mutation.error),
    remove: (id: string) => mutation.mutateAsync({ id }),
  }
}
