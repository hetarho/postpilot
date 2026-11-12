import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { normalizeAppFailure } from '@/shared/api'
import type { ClipEditPlan, RetainedClipSource } from '@/entities/clip-plan'
import {
  BrowserCompositionOriginals,
  canonicalBrowserCompositionData,
  freezeBrowserPreviewComposition,
  projectBrowserComposition,
  type BrowserCompositionDesign,
  type BrowserLocalOriginal,
  type ClipLocalCompositionRuntime,
  type ClipPreviewOverlay,
} from '@/entities/clip-preview'
import type { ClipLayoutObservations } from '@/entities/clip-observation'
import { CLIP_DESIGN, type ClipRatioId } from '@/entities/clip-design'

export interface ClipDraftPreviewInput {
  ownerId: string
  projectId: string
  projectRevision?: number
  revision: number
  plan: ClipEditPlan
  ratio: ClipRatioId
  sources: readonly RetainedClipSource[]
  layoutObservations?: ClipLayoutObservations
  design?: Partial<BrowserCompositionDesign>
  localSources?: readonly BrowserLocalOriginal[]
  resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>
  timeMs?: number
}

function localAccessIdentity(source: BrowserLocalOriginal) {
  const hint = source.retainedAccess
  if (
    !source.file &&
    hint &&
    typeof hint.state === 'string' &&
    Number.isSafeInteger(hint.generation) &&
    hint.generation >= 0
  ) {
    const url = source.url ?? ''
    let http = false
    try {
      http = ['http:', 'https:'].includes(new URL(url, 'https://retained.invalid').protocol)
    } catch {
      /* Unknown access shapes keep the raw URL fence. */
    }
    if ((!url && hint.generation !== 1) || (http && url && hint.generation > 0))
      return ['retained', Math.max(0, hint.generation - 1), hint.state]
  }
  return source.url
}

type RetainedHint = BrowserLocalOriginal['retainedAccess']
interface ContinuityInput {
  content: string
  access: unknown[][]
  authority: Array<{ hint?: RetainedHint; url?: string }>
  authorityKeys: string[]
}
function futureLease(hint: RetainedHint, now: number) {
  const lease = hint?.lease
  if (
    !hint ||
    !Number.isSafeInteger(hint.generation) ||
    hint.generation < 0 ||
    !lease ||
    lease.version !== 1 ||
    lease.confirmed !== true ||
    typeof lease.batchId !== 'string' ||
    !lease.batchId ||
    lease.availability !== 'available' ||
    lease.playbackError !== undefined ||
    hint.state !==
      JSON.stringify([lease.availability, lease.retentionExpiresAt, lease.playbackError])
  )
    return undefined
  const value = lease.retentionExpiresAt
  // The server emits UTC RFC3339Nano. Reject normalized invalid dates and retain
  // nanosecond-shaped timestamps; sub-millisecond-only increases fail closed.
  if (
    typeof value !== 'string' ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/u.test(value)
  )
    return undefined
  const at = Date.parse(value)
  if (
    !Number.isFinite(at) ||
    at <= now ||
    new Date(at).toISOString().slice(0, 19) !== value.slice(0, 19)
  )
    return undefined
  return at
}
function leaseExtension(
  before: ContinuityInput,
  after: ContinuityInput,
  index: number,
  now: number,
) {
  const old = before.authority[index],
    next = after.authority[index]
  const a = old?.hint,
    b = next?.hint
  const oldAt = futureLease(a, now),
    nextAt = futureLease(b, now)
  if (
    !oldAt ||
    !nextAt ||
    nextAt <= oldAt ||
    !a ||
    !b ||
    a.generation !== b.generation ||
    !old.url ||
    old.url !== next.url ||
    before.access[index]?.[3] ||
    after.access[index]?.[3]
  )
    return false
  const oldAccess = before.access[index]?.[2],
    nextAccess = after.access[index]?.[2]
  if (
    !Array.isArray(oldAccess) ||
    !Array.isArray(nextAccess) ||
    oldAccess[0] !== 'retained' ||
    nextAccess[0] !== 'retained'
  )
    return false
  const withoutExpiry = (hint: NonNullable<RetainedHint>) => ({
    ...hint,
    state: undefined,
    lease: { ...hint.lease, retentionExpiresAt: undefined },
  })
  return (
    canonicalBrowserCompositionData(withoutExpiry(a)) ===
      canonicalBrowserCompositionData(withoutExpiry(b)) &&
    canonicalBrowserCompositionData(before.access[index]?.slice(0, 2)) ===
      canonicalBrowserCompositionData(after.access[index]?.slice(0, 2))
  )
}
function compatibleContinuity(before: ContinuityInput, after: ContinuityInput, now: number) {
  if (before.content !== after.content || before.access.length !== after.access.length)
    return undefined
  let extended = false
  for (let i = 0; i < after.access.length; i++) {
    const sameAccess =
      canonicalBrowserCompositionData(before.access[i]) ===
      canonicalBrowserCompositionData(after.access[i])
    if (sameAccess && before.authorityKeys[i] === after.authorityKeys[i]) continue
    const a = before.authority[i],
      b = after.authority[i]
    // T780's first publication is already the same access identity. Do not turn
    // it into a new cancellation cause; every later capability change still is.
    if (
      sameAccess &&
      !a?.url &&
      b?.url &&
      a.hint?.generation === 0 &&
      b.hint?.generation === 1 &&
      canonicalBrowserCompositionData({ ...a.hint, generation: undefined }) ===
        canonicalBrowserCompositionData({ ...b.hint, generation: undefined })
    )
      continue
    if (!leaseExtension(before, after, i, now)) return undefined
    extended = true
  }
  return { extended }
}

const retryScope = (data: ClipDraftPreviewInput) =>
  JSON.stringify([
    data.ownerId,
    data.projectId,
    data.projectRevision ?? data.revision,
    data.revision,
    data.plan.cuts.map((cut) => [cut.sourceId, cut.fingerprint]),
  ])
const retrySources = (data: ClipDraftPreviewInput) =>
  data.plan.cuts.map((cut) => {
    const local = data.localSources?.find(
      (source) => source.sourceId === cut.sourceId && source.fingerprint === cut.fingerprint,
    )
    return {
      sourceId: cut.sourceId,
      fingerprint: cut.fingerprint,
      file: local?.file,
      present: !!local,
      blob: local?.url?.startsWith('blob:') ? local.url : undefined,
    }
  })
type RetryRequest =
  { status: 'succeeded' } | { status: 'pending' | 'failed'; promise: Promise<string> }
interface ExplicitRetry {
  scope: string
  resolver: ClipDraftPreviewInput['resolvePlayback']
  sources: ReturnType<typeof retrySources>
  requests: Map<string, RetryRequest>
}
function matchesRetry(attempt: ExplicitRetry, data: ClipDraftPreviewInput) {
  const sources = retrySources(data)
  return (
    attempt.scope === retryScope(data) &&
    attempt.resolver === data.resolvePlayback &&
    attempt.sources.every(
      (source, index) =>
        source.file === sources[index]?.file &&
        source.blob === sources[index]?.blob &&
        source.present === sources[index]?.present,
    )
  )
}
/** Page ownership around the shared local engine. Scrubbing never requests a server raster. */
export function useClipDraftPreview(
  input: ClipDraftPreviewInput,
): ClipPreviewOverlay & { local?: ClipLocalCompositionRuntime } {
  const [retry, setRetry] = useState(0)
  const explicitRetry = useRef<ExplicitRetry | undefined>(undefined)
  const nextFiles = (input.localSources ?? []).map((source) => source.file)
  const [files, setFiles] = useState(nextFiles)
  if (files.length !== nextFiles.length || files.some((file, index) => file !== nextFiles[index]))
    setFiles(nextFiles)
  const key = JSON.stringify([
    input.ownerId,
    input.projectId,
    input.projectRevision ?? input.revision,
    input.revision,
    input.plan,
    input.ratio,
    input.sources,
    input.layoutObservations,
    input.design,
    (input.localSources ?? []).map((source) => [
      source.sourceId,
      source.fingerprint,
      localAccessIdentity(source),
      !!source.file,
    ]),
    retry,
  ])
  // The raw typed authority is observed even when T780's opaque full key is
  // unchanged (batch/confirmation/version/binding). No deep sort on seeks.
  const authorityKey = JSON.stringify(
    (input.localSources ?? []).map((source) => ({
      hint: source.retainedAccess,
      url: source.url,
    })),
  )
  const comparison = useMemo<ContinuityInput>(() => {
    const values = JSON.parse(key) as unknown[]
    const authority = JSON.parse(authorityKey) as ContinuityInput['authority']
    return {
      content: canonicalBrowserCompositionData([
        values[0],
        values[1],
        ...values.slice(4, 9),
        values[10],
      ]),
      access: values[9] as unknown[][],
      authority,
      authorityKeys: authority.map((value) => canonicalBrowserCompositionData(value)),
    }
  }, [key, authorityKey])
  const continuity = useRef<
    | {
        controller: AbortController
        input: ContinuityInput
        files: typeof files
        resolver: ClipDraftPreviewInput['resolvePlayback']
        leaseEpoch: number
      }
    | undefined
  >(undefined)
  const [state, setState] = useState<{
    key: string
    local?: ClipLocalCompositionRuntime
    error?: unknown
    owner?: AbortSignal
    files?: typeof files
    resolver?: ClipDraftPreviewInput['resolvePlayback']
  }>({ key: '' })
  const latest = useRef(input)
  useLayoutEffect(() => {
    latest.current = input
    const before = continuity.current
    if (
      !before ||
      before.controller.signal.aborted ||
      before.input !== comparison ||
      before.files !== files ||
      before.resolver !== input.resolvePlayback
    ) {
      const compatible =
        before &&
        !before.controller.signal.aborted &&
        before.files === files &&
        before.resolver === input.resolvePlayback
          ? compatibleContinuity(before.input, comparison, Date.now())
          : undefined
      if (compatible && before) {
        before.input = comparison
        if (compatible.extended) before.leaseEpoch++
      } else {
        before?.controller.abort()
        continuity.current = {
          controller: new AbortController(),
          input: comparison,
          files,
          resolver: input.resolvePlayback,
          leaseEpoch: 0,
        }
      }
    }
    if (explicitRetry.current && !matchesRetry(explicitRetry.current, input))
      explicitRetry.current = undefined
  }, [input, comparison, files])
  useLayoutEffect(() => () => continuity.current?.controller.abort(), [])
  useEffect(() => {
    const data = latest.current
    const owner = continuity.current!
    const expires = owner.input.authority
      .map((value) => futureLease(value.hint, -Infinity))
      .filter((value): value is number => value !== undefined)
    const marker = {
      signal: owner.controller.signal,
      leaseEpoch: owner.leaseEpoch,
      retainedUntil: expires.length ? Math.min(...expires) : undefined,
    }
    const controller = new AbortController()
    let originals: BrowserCompositionOriginals | undefined
    void freezeBrowserPreviewComposition(
      projectBrowserComposition({
        ownerId: data.ownerId,
        projectId: data.projectId,
        projectRevision: data.projectRevision ?? data.revision,
        planRevision: data.revision,
        plan: data.plan,
        ratio: data.ratio,
        sources: data.sources,
        layoutObservations: data.layoutObservations,
        design: data.design,
      }),
    ).then(
      (snapshot) => {
        if (controller.signal.aborted) return
        originals = new BrowserCompositionOriginals(
          snapshot,
          data.localSources ?? [],
          (fingerprint, refresh = false) => {
            if (refresh) return data.resolvePlayback(fingerprint, true)
            const attempt = explicitRetry.current
            const used = attempt?.sources.find((source) => source.fingerprint === fingerprint)
            if (!attempt || !used || !matchesRetry(attempt, data))
              return data.resolvePlayback(fingerprint, false)
            const id = JSON.stringify([used.sourceId, fingerprint])
            const previous = attempt.requests.get(id)
            if (previous?.status === 'succeeded') return data.resolvePlayback(fingerprint, false)
            if (previous) return previous.promise
            // Consume before invoking the resolver: synchronous publication or
            // another owner shares pending work and cannot repeat a force.
            const request: RetryRequest = {
              status: 'pending',
              promise: Promise.resolve().then(() => data.resolvePlayback(fingerprint, true)),
            }
            attempt.requests.set(id, request)
            void request.promise.then(
              () => {
                attempt.requests.set(id, { status: 'succeeded' })
              },
              () => {
                request.status = 'failed'
              },
            )
            return request.promise
          },
          controller.signal,
        )
        setState({
          key,
          owner: controller.signal,
          files,
          resolver: data.resolvePlayback,
          local: {
            snapshot,
            continuity: marker,
            source: originals.source,
            audioSource: originals.audioSource,
          },
        })
      },
      (error: unknown) => {
        if (!controller.signal.aborted) {
          owner.controller.abort()
          setState({ key, owner: controller.signal, files, resolver: data.resolvePlayback, error })
        }
      },
    )
    return () => {
      controller.abort()
      originals?.dispose()
    }
    // `key` is the detached content/runtime identity; output playhead changes preserve it.
  }, [key, retry, files, input.resolvePlayback])
  const current =
      state.key === key &&
      !state.owner?.aborted &&
      state.files === files &&
      state.resolver === input.resolvePlayback
        ? state
        : undefined,
    canvas = CLIP_DESIGN.ratios[input.ratio].canvas
  return {
    local: current?.local,
    assets: [],
    canvasWidth: canvas.width,
    canvasHeight: canvas.height,
    ready: !!current?.local,
    updating: !current,
    failure: current?.error
      ? normalizeAppFailure({
          reason:
            current.error instanceof Error
              ? current.error.message.split(':')[0]!
              : 'CLIP_PREVIEW_UNAVAILABLE',
          params: {},
        })
      : undefined,
    onRetry: useCallback(() => {
      const data = latest.current
      continuity.current?.controller.abort()
      explicitRetry.current = {
        scope: retryScope(data),
        resolver: data.resolvePlayback,
        sources: retrySources(data),
        requests: new Map(),
      }
      setRetry((value) => value + 1)
    }, []),
  }
}
