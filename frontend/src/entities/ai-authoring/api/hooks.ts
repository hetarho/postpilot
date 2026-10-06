import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { createConnectQueryKey, useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ConfigurationAuthoringService as Service } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  authoringKindToProto,
  authoringModeToProto,
  mapAuthoringEstimate,
  mapAuthoringSession,
  type WireSession,
} from './mappers'
import {
  authoringSessionBusy,
  type AuthoringScope,
  type AuthoringSession,
  type AuthoringMode,
  type AuthoringModelRef,
  type AuthoringStart,
} from '../model/types'

export function authoringLatestQueryKey(transport: Transport, scope: AuthoringScope) {
  return [
    ...createConnectQueryKey({
      schema: Service.method.getLatestAuthoringSession,
      input: { kind: authoringKindToProto(scope.kind), targetId: scope.targetId ?? '' },
      transport,
      cardinality: 'finite',
    }),
    scope.ownerId,
  ] as const
}
export function authoringSessionQueryKey(transport: Transport, scope: AuthoringScope, id: string) {
  return [
    ...createConnectQueryKey({
      schema: Service.method.getAuthoringSession,
      input: { sessionId: id },
      transport,
      cardinality: 'finite',
    }),
    scope.ownerId,
    scope.kind,
    scope.targetId ?? '',
  ] as const
}
export function useAuthoringAPI(scope: AuthoringScope) {
  const transport = useTransport()
  const cache = useQueryClient()
  const { ownerId, kind, targetId } = scope
  const ownedScope = useMemo(() => ({ ownerId, kind, targetId }), [ownerId, kind, targetId])
  return useMemo(() => {
    const client = createClient(Service, transport)
    const publish = (wire: WireSession | undefined, expectedId?: string) => {
      if (!wire) throw new Error('Authoring response unavailable')
      if (expectedId && wire.id !== expectedId) throw new Error('Authoring session mismatch')
      const session = mapAuthoringSession(wire, ownedScope)
      const key = authoringSessionQueryKey(transport, ownedScope, session.id)
      cache.setQueryData<AuthoringSession>(key, (previous) =>
        previous && previous.revision > session.revision ? previous : session,
      )
      cache.setQueryData<AuthoringSession | null>(
        authoringLatestQueryKey(transport, ownedScope),
        (previous) =>
          previous?.id === session.id && previous.revision > session.revision ? previous : session,
      )
      return session
    }
    return {
      create: async (requestId: string) =>
        publish(
          (
            await client.createAuthoringSession({
              kind: authoringKindToProto(ownedScope.kind),
              targetId: ownedScope.targetId ?? '',
              requestId,
            })
          ).session,
        ),
      load: async (sessionId: string) =>
        publish((await client.getAuthoringSession({ sessionId })).session, sessionId),
      estimate: async (mode: AuthoringMode, writeModel: AuthoringModelRef, sessionId = '') =>
        mapAuthoringEstimate(
          await client.estimateAuthoringOperation({
            kind: authoringKindToProto(ownedScope.kind),
            mode: authoringModeToProto(mode),
            writeModel,
            sessionId,
          }),
        ),
      start: async (input: AuthoringStart) => {
        const response = await client.startAuthoringOperation({
          sessionId: input.sessionId,
          expectedRevision: input.expectedRevision,
          requestId: input.requestId,
          mode: authoringModeToProto(input.mode),
          prompt: input.prompt,
          writeModel: input.writeModel,
        })
        if (!response.jobId) throw new Error('Authoring job unconfirmed')
        return { jobId: response.jobId, session: publish(response.session, input.sessionId) }
      },
      select: async (sessionId: string, expectedRevision: number, candidateId: string) =>
        publish(
          (await client.selectAuthoringCandidate({ sessionId, expectedRevision, candidateId }))
            .session,
          sessionId,
        ),
      cancel: async (sessionId: string, jobId: string) =>
        publish((await client.cancelAuthoringOperation({ sessionId, jobId })).session, sessionId),
      save: async (sessionId: string, expectedRevision: number, makeDefault: boolean) => {
        const session = publish(
          (await client.saveAuthoringSession({ sessionId, expectedRevision, makeDefault })).session,
          sessionId,
        )
        if (session.phase !== 'saved' || !session.saved)
          throw new Error('Authoring save unconfirmed')
        return session
      },
    }
  }, [cache, transport, ownedScope])
}
export function useLatestAuthoringSession(scope: AuthoringScope) {
  const cache = useQueryClient()
  const transport = useTransport()
  const client = useMemo(() => createClient(Service, transport), [transport])
  return useQuery({
    queryKey: authoringLatestQueryKey(transport, scope),
    queryFn: async ({ signal }) => {
      const response = await client.getLatestAuthoringSession(
        { kind: authoringKindToProto(scope.kind), targetId: scope.targetId ?? '' },
        { signal },
      )
      const incoming = response.session ? mapAuthoringSession(response.session, scope) : null
      const previous = cache.getQueryData<AuthoringSession | null>(
        authoringLatestQueryKey(transport, scope),
      )
      return previous &&
        (!incoming || (previous.id === incoming.id && previous.revision > incoming.revision))
        ? previous
        : incoming
    },
    enabled: scope.ownerId !== '',
    retry: false,
    refetchOnWindowFocus: false,
  })
}
export function useAuthoringSession(scope: AuthoringScope, id: string) {
  const cache = useQueryClient()
  const transport = useTransport()
  const client = useMemo(() => createClient(Service, transport), [transport])
  const queryKey = authoringSessionQueryKey(transport, scope, id)
  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }) => {
      const response = await client.getAuthoringSession({ sessionId: id }, { signal })
      if (!response.session) throw new Error('Authoring session unavailable')
      if (response.session.id !== id) throw new Error('Authoring session mismatch')
      const incoming = mapAuthoringSession(response.session, scope)
      const previous = cache.getQueryData<AuthoringSession>(queryKey)
      return previous && previous.revision > incoming.revision ? previous : incoming
    },
    enabled: scope.ownerId !== '' && id !== '',
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => (authoringSessionBusy(query.state.data) ? POLL_INTERVAL_MS : false),
  })
  return { ...query, queryKey }
}
