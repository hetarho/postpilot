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
  mapAuthoringSummary,
  type WireSession,
} from './mappers'
import {
  authoringSessionBusy,
  type AuthoringScope,
  type AuthoringSession,
  type AuthoringSummary,
  type AuthoringMode,
  type AuthoringModelRef,
  type AuthoringStart,
  type AuthoringArtifact,
  type AuthoringCandidateCount,
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
      void cache.invalidateQueries({
        queryKey: ['authoring-summaries', ownedScope.ownerId, ownedScope.kind],
      })
      return session
    }
    return {
      create: async (requestId: string, referencePost = '') =>
        publish(
          (
            await client.createAuthoringSession({
              kind: authoringKindToProto(ownedScope.kind),
              targetId: ownedScope.targetId ?? '',
              requestId,
              referencePost,
            })
          ).session,
        ),
      load: async (sessionId: string) =>
        publish((await client.getAuthoringSession({ sessionId })).session, sessionId),
      estimate: async (
        mode: AuthoringMode,
        writeModel: AuthoringModelRef,
        sessionId = '',
        candidateCount: AuthoringCandidateCount = 8,
      ) =>
        mapAuthoringEstimate(
          await client.estimateAuthoringOperation({
            kind: authoringKindToProto(ownedScope.kind),
            mode: authoringModeToProto(mode),
            writeModel,
            sessionId,
            candidateCount,
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
          candidateCount: input.candidateCount ?? 8,
        })
        if (!response.jobId) throw new Error('Authoring job unconfirmed')
        return { jobId: response.jobId, session: publish(response.session, input.sessionId) }
      },
      select: async (
        sessionId: string,
        expectedRevision: number,
        candidateId: string,
        operationKey: string = crypto.randomUUID(),
      ) =>
        publish(
          (
            await client.selectAuthoringCandidate({
              sessionId,
              expectedRevision,
              candidateId,
              operationKey,
            })
          ).session,
          sessionId,
        ),
      patch: async (
        sessionId: string,
        expectedRevision: number,
        operationKey: string,
        workingSource: AuthoringArtifact,
      ) =>
        publish(
          (
            await client.patchAuthoringDraft({
              sessionId,
              expectedRevision,
              operationKey,
              workingSource,
            })
          ).session,
          sessionId,
        ),
      resetChat: async (sessionId: string, expectedRevision: number, operationKey: string) =>
        publish(
          (await client.resetAuthoringChat({ sessionId, expectedRevision, operationKey })).session,
          sessionId,
        ),
      resetBaseline: async (sessionId: string, expectedRevision: number, operationKey: string) =>
        publish(
          (await client.resetAuthoringBaseline({ sessionId, expectedRevision, operationKey }))
            .session,
          sessionId,
        ),
      cancel: async (sessionId: string, jobId: string) =>
        publish((await client.cancelAuthoringOperation({ sessionId, jobId })).session, sessionId),
      save: async (
        sessionId: string,
        expectedRevision: number,
        makeDefault: boolean,
        operationKey: string = crypto.randomUUID(),
      ) => {
        const session = publish(
          (
            await client.saveAuthoringSession({
              sessionId,
              expectedRevision,
              makeDefault,
              operationKey,
            })
          ).session,
          sessionId,
        )
        if (session.phase !== 'saved' || !session.saved)
          throw new Error('Authoring save unconfirmed')
        return session
      },
    }
  }, [cache, transport, ownedScope])
}
export function useLatestAuthoringSession(scope: AuthoringScope, enabled = true) {
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
    enabled: scope.ownerId !== '' && enabled,
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

export function useAuthoringSummaries(scope: AuthoringScope, unsavedOnly = false) {
  const transport = useTransport()
  const client = useMemo(() => createClient(Service, transport), [transport])
  return useQuery<AuthoringSummary[]>({
    queryKey: [
      'authoring-summaries',
      scope.ownerId,
      scope.kind,
      ...createConnectQueryKey({
        schema: Service.method.listAuthoringSummaries,
        input: { kind: authoringKindToProto(scope.kind), unsavedOnly, pageSize: 100 },
        transport,
        cardinality: 'finite',
      }),
    ],
    enabled: !!scope.ownerId,
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) =>
      query.state.data?.some((summary) => summary.activeJobId || summary.publicationPending)
        ? POLL_INTERVAL_MS
        : false,
    queryFn: async ({ signal }) => {
      const summaries = []
      let pageToken = ''
      const seen = new Set<string>()
      do {
        if (seen.has(pageToken)) throw new Error('Authoring summary cursor unavailable')
        seen.add(pageToken)
        const response = await client.listAuthoringSummaries(
          { kind: authoringKindToProto(scope.kind), unsavedOnly, pageSize: 100, pageToken },
          { signal },
        )
        summaries.push(
          ...response.summaries.map((summary) => mapAuthoringSummary(summary, scope.kind)),
        )
        if (response.nextPageToken === pageToken && pageToken)
          throw new Error('Authoring summary cursor unavailable')
        pageToken = response.nextPageToken
      } while (pageToken)
      return summaries
    },
  })
}
