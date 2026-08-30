import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { publishingClientFor, type ProtoPublishingAgent } from '@/shared/api'
import { PUBLISH_AGENT_STALE_MS } from '@/shared/config'
import type { PublishingAgent } from '../model/types'

export const publishingAgentsQueryKey = (transport: Transport, ownerId: string) =>
  ['publishing-agents', ownerId, transport] as const

export function toPublishingAgent(agent: ProtoPublishingAgent): PublishingAgent {
  return {
    id: agent.id,
    label: agent.label,
    platformAccountId: agent.platformAccountId,
    platformAccountLabel: agent.platformAccountLabel,
    browserLabel: agent.browserLabel,
    categories: agent.categories.map((category) => ({ id: category.id, name: category.name })),
    defaultCategoryId: agent.defaultCategoryId,
    defaultVisibility: agent.defaultVisibility,
    lastSeenAt: agent.lastSeenAt,
    revokedAt: agent.revokedAt,
    ready: agent.ready,
  }
}

export function usePublishingAgents(ownerId: string) {
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const query = useQuery({
    queryKey: publishingAgentsQueryKey(transport, ownerId),
    queryFn: () => client.listPublishingAgents({}),
    enabled: Boolean(ownerId),
    refetchInterval: ownerId ? PUBLISH_AGENT_STALE_MS : false,
  })
  return {
    agents: query.data?.agents.map(toPublishingAgent) ?? [],
    observedAt: query.dataUpdatedAt,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}
