import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidateGuidelines } from '@/entities/guideline'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidatePurposes, purposeErrorMessage } from '@/entities/purpose'
import { PurposeService } from '@/shared/api'

/** Presence is the edit unit: each saver sends exactly one field, so two fields edited from two
 *  tabs cannot overwrite each other (spec/policy/purposes.md). Sending all three every time
 *  would be a read-modify-write and would put back whatever the other tab had just changed. */
export function useUpdatePurpose(ownerId: string, purposeId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PurposeService.method.updatePurpose, {
    onSuccess: (_data, saved) => {
      invalidatePurposes(queryClient, transport, ownerId)
      // A rename changes no post row, only what every one of them DISPLAYS: the name is
      // projected onto each post as `purpose.name` and rendered on the list. Without this
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
    errorMessage: purposeErrorMessage(mutation.error),
    saveName: (name: string) => mutation.mutateAsync({ id: purposeId, name: name.trim() }),
    saveDescription: (description: string) =>
      mutation.mutateAsync({ id: purposeId, description: description.trim() }),
    saveInstructions: (instructions: string) =>
      mutation.mutateAsync({ id: purposeId, instructions: instructions.trim() }),
  }
}
