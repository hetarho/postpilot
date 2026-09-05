import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidateGuidelines } from '@/entities/guideline'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateTemplates, templateErrorMessage } from '@/entities/template'
import { TemplateService } from '@/shared/api'

/** Presence is the edit unit. `saveAll` sends the three fields the template screen edits as one
 *  draft, which is one transaction on the server rather than a read-modify-write: the screen is
 *  the only place all three are edited, so there is no other tab's value for it to put back
 *  (spec/policy/templates.md). */
export function useUpdateTemplate(ownerId: string, templateId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(TemplateService.method.updateTemplate, {
    onSuccess: (_data, saved) => {
      invalidateTemplates(queryClient, transport, ownerId)
      // A rename changes no post row, only what every one of them DISPLAYS: the name is
      // projected onto each post as `template.name` and rendered on the list. Without this
      // the badges keep the old name until something else happens to refetch them — the
      // same reason `useRenameVoice` invalidates these two keys for the identical projection.
      if (saved.name !== undefined) {
        void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
        void queryClient.invalidateQueries({ queryKey: postDetailQueriesKey(transport) })
        // The guideline list projects the same name onto every scope chip, for the same reason.
        invalidateGuidelines(queryClient, transport, ownerId)
      }
    },
  })
  return {
    ...mutation,
    errorMessage: templateErrorMessage(mutation.error),
    saveAll: (fields: { name: string; description: string; body: string }) =>
      mutation.mutateAsync({
        id: templateId,
        name: fields.name.trim(),
        description: fields.description.trim(),
        // NOT trimmed: the body is the canonical serialization of the composition, and trimming
        // it here would rewrite a stored body that carries significant outer bytes (change 30 A11).
        body: fields.body,
      }),
  }
}
