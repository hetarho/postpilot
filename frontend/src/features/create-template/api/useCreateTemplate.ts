import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidateTemplates, templateErrorMessage } from '@/entities/template'
import { TemplateService } from '@/shared/api'

export function useCreateTemplate(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(TemplateService.method.createTemplate, {
    // A new template changes only the directory: no post is assigned to it yet, and nothing
    // about it is learned or generated ([I5]).
    onSuccess: () => invalidateTemplates(queryClient, transport, ownerId),
  })
  return {
    ...mutation,
    errorMessage: templateErrorMessage(mutation.error),
    create: (fields: { name: string; description: string; body: string }) =>
      mutation.mutateAsync(fields),
  }
}
