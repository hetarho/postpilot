import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, publishingClientFor, type PublishVisibility } from '@/shared/api'
import { publishingAgentsQueryKey } from './queries'

/** The three writes on the paired Mac companion. Each of them changes only the agent directory,
 *  so they live with the agent noun (ARCH-14) and the forms stay in their verb features. */

export function usePairPublishingAgent(ownerId: string) {
  const client = usePublishingClient()
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (label: string) => client.createAgentPairing({ label: label.trim() }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(ownerId) }),
  })
  return { ...mutation, failure: failureOf(mutation.error) }
}

export function useConfigurePublishingAgent(ownerId: string, agentId: string) {
  const client = usePublishingClient()
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (input: {
      label: string
      defaultCategoryId: string
      defaultVisibility: PublishVisibility
    }) =>
      client.updatePublishingAgent({
        agentId,
        label: input.label.trim(),
        defaultCategoryId: input.defaultCategoryId,
        defaultVisibility: input.defaultVisibility,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(ownerId) }),
  })
  return { ...mutation, failure: failureOf(mutation.error) }
}

export function useRevokePublishingAgent(ownerId: string, agentId: string) {
  const client = usePublishingClient()
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: () => client.revokePublishingAgent({ agentId }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(ownerId) }),
  })
  return { ...mutation, failure: failureOf(mutation.error) }
}

function usePublishingClient() {
  const transport = useTransport()
  return useMemo(() => publishingClientFor(transport), [transport])
}

function failureOf(error: Error | null) {
  return error ? appFailureFromConnect(error) : undefined
}
