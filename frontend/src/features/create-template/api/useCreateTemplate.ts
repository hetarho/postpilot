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
    // The two generation numbers ride along, `undefined` meaning 의견 없음 - the proto field is
    // optional, so an unticked one is simply not on the wire (TEMPLATE-47).
    create: (fields: {
      name: string
      description: string
      body: string
      targetLength?: number
      tagCount?: number
    }) => mutation.mutateAsync(fields),
  }
}
