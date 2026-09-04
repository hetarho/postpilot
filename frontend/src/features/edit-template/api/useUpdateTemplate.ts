import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidateGuidelines } from '@/entities/guideline'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateTemplates, templateErrorMessage } from '@/entities/template'
import { TemplateService } from '@/shared/api'

/** Presence is the edit unit: each saver sends exactly one field, so two fields edited from two
 *  tabs cannot overwrite each other (spec/policy/templates.md). Sending all three every time
 *  would be a read-modify-write and would put back whatever the other tab had just changed. */
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
    saveName: (name: string) => mutation.mutateAsync({ id: templateId, name: name.trim() }),
    saveDescription: (description: string) =>
      mutation.mutateAsync({ id: templateId, description: description.trim() }),
    saveBody: (body: string) => mutation.mutateAsync({ id: templateId, body: body.trim() }),
  }
}
