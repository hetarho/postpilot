import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react'
import { Code, ConnectError, type Transport } from '@connectrpc/connect'
import { createConnectQueryKey, useTransport } from '@connectrpc/connect-query'
import { hashKey, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import {
  AuthService,
  WritingInspectionService,
  appFailureFromConnect,
  onUnauthenticated,
  type GetMeResponse,
} from '@/shared/api'
import { REQUEST_INSPECTION_MAX_TIMER_MS } from '../config/limits'
import {
  requestInspectionTargetKey,
  type RequestInspectionSelection,
  type RequestInspectionTarget,
} from '../model/types'
import { createRequestInspectionReader } from './client'

function authenticatedOwner(cache: QueryClient, transport: Transport): string {
  const queryKey = createConnectQueryKey({
    schema: AuthService.method.getMe,
    input: {},
    transport,
    cardinality: 'finite',
  })
  const state = cache.getQueryState<GetMeResponse>(queryKey)
  return state?.status === 'success' ? (state.data?.user?.id ?? '') : ''
}

function resource(target: RequestInspectionTarget): string {
  switch (target.kind) {
    case 'post':
      return target.postSlug
    case 'authoring':
      return target.sessionId
    case 'test':
      return target.testId
  }
}

export function requestInspectionQueryKey(
  transport: Transport,
  target: RequestInspectionTarget | null,
  selection: RequestInspectionSelection,
  enabled = true,
  authenticatedOwnerId = target?.ownerId ?? '',
) {
  const scopedTarget = enabled ? target : null
  return [
    'request-inspection',
    scopedTarget?.ownerId ?? '',
    scopedTarget?.kind ?? '',
    scopedTarget ? resource(scopedTarget) : '',
    requestInspectionTargetKey(scopedTarget),
    selection.stage,
    selection.status,
    enabled ? 'open' : 'closed',
    authenticatedOwnerId,
    ...createConnectQueryKey({
      schema: WritingInspectionService.method.getPostRequestInspection,
      transport,
      cardinality: 'finite',
    }),
  ] as const
}

function expiry(target: RequestInspectionTarget | null): number | undefined {
  if (target?.kind !== 'test' || !target.payloadExpiresAt) return undefined
  const value = Date.parse(target.payloadExpiresAt)
  // An unknown retention timestamp cannot grant access to potentially expired material.
  return Number.isFinite(value) ? value : 0
}

function removalFence(cache: QueryClient, key: readonly unknown[]) {
  let removed = false
  const hash = hashKey(key)
  return {
    snapshot: () => removed,
    subscribe: (changed: () => void) =>
      cache.getQueryCache().subscribe((event) => {
        if (event.type === 'removed' && event.query.queryHash === hash) {
          removed = true
          changed()
        }
      }),
  }
}

/** Private material lives only while the named view is open and its owner/fences agree. */
export function useRequestInspection(
  target: RequestInspectionTarget | null,
  selection: RequestInspectionSelection,
  enabled = true,
) {
  const transport = useTransport()
  const cache = useQueryClient()
  const read = useMemo(() => createRequestInspectionReader(transport), [transport])
  const subscribe = useCallback(
    (changed: () => void) => cache.getQueryCache().subscribe(changed),
    [cache],
  )
  const ownerSnapshot = useCallback(() => authenticatedOwner(cache, transport), [cache, transport])
  const ownerId = useSyncExternalStore(subscribe, ownerSnapshot, ownerSnapshot)
  const [now, setNow] = useState(() => Date.now())
  const expiresAt = expiry(target)
  const expired = expiresAt !== undefined && now >= expiresAt
  const targetKey = requestInspectionTargetKey(target)
  const queryKey = useMemo(
    () => requestInspectionQueryKey(transport, target, selection, enabled, ownerId),
    // These scalar keys contain every target/input field and the chosen read mode.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [transport, targetKey, selection.stage, selection.status, enabled, ownerId],
  )
  // A removed active observer must not reopen the same private query automatically.
  // Only another explicit open, owner change or resource/input identity gets a new fence.
  const fence = useMemo(() => removalFence(cache, queryKey), [cache, queryKey])
  const removed = useSyncExternalStore(fence.subscribe, fence.snapshot, fence.snapshot)
  const allowed = enabled && !!target?.ownerId && target.ownerId === ownerId && !expired && !removed
  const query = useQuery({
    queryKey,
    enabled: allowed,
    retry: false,
    staleTime: 0,
    // Explicit scope cleanup removes the query. A zero-time collector can remove a
    // newly reopened query through the old observer's delayed collection callback.
    gcTime: Infinity,
    refetchOnWindowFocus: false,
    queryFn: async ({ signal }) => {
      if (!allowed || !target || authenticatedOwner(cache, transport) !== target.ownerId)
        throw new ConnectError('Inspection owner unavailable', Code.Unauthenticated)
      const originalQuery = cache.getQueryCache().find({ queryKey, exact: true })
      const capturedExpiry = expiry(target)
      if (capturedExpiry !== undefined && Date.now() >= capturedExpiry)
        throw new ConnectError('Inspection payload unavailable', Code.NotFound)
      const result = await read(target, selection, signal)
      if (
        signal.aborted ||
        authenticatedOwner(cache, transport) !== target.ownerId ||
        cache.getQueryCache().find({ queryKey, exact: true }) !== originalQuery ||
        (capturedExpiry !== undefined && Date.now() >= capturedExpiry)
      )
        throw new ConnectError('Inspection scope ended', Code.Canceled)
      return result
    },
  })

  useEffect(() => {
    if (!allowed) {
      void cache.cancelQueries({ queryKey, exact: true })
      cache.removeQueries({ queryKey, exact: true })
    }
    return () => {
      void cache.cancelQueries({ queryKey, exact: true })
      cache.removeQueries({ queryKey, exact: true })
    }
  }, [allowed, cache, queryKey])

  useEffect(
    () =>
      onUnauthenticated(() => {
        void cache.cancelQueries({ queryKey: ['request-inspection'] })
        cache.removeQueries({ queryKey: ['request-inspection'] })
      }),
    [cache],
  )

  useEffect(() => {
    if (!allowed || expiresAt === undefined) return
    const timer = setTimeout(
      () => setNow(Date.now()),
      Math.min(Math.max(0, expiresAt - Date.now()), REQUEST_INSPECTION_MAX_TIMER_MS),
    )
    return () => clearTimeout(timer)
  }, [allowed, expiresAt, now])

  return {
    ...query,
    // An observer can retain its old data briefly after QueryCache.remove. Recheck scope
    // on every render as well as before/after the awaited read.
    data: allowed ? query.data : undefined,
    isPending: allowed && query.isPending,
    scopeAvailable: !!target?.ownerId && target.ownerId === ownerId && !expired && !removed,
    failure: allowed && query.error ? appFailureFromConnect(query.error) : undefined,
    queryKey,
  }
}
