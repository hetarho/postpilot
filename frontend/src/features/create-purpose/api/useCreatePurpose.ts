import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidatePurposes, purposeErrorMessage } from '@/entities/purpose'
import { PurposeService } from '@/shared/api'

export function useCreatePurpose(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PurposeService.method.createPurpose, {
    // A new purpose changes only the directory: no post is assigned to it yet, and nothing
    // about it is learned or generated ([I5]).
    onSuccess: () => invalidatePurposes(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: purposeErrorMessage(mutation.error),
    create: (fields: { name: string; description: string; instructions: string }) =>
      mutation.mutateAsync(fields),
  }
}
