import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  clipProjectsKey,
  toClipProject,
  useClipAnalysisEligibility,
  type ClipProject,
  type ClipQuote,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import { isTerminal, useJob } from '@/entities/generation-job'
import { myPlanQueryKey } from '@/entities/plan'
import {
  useSelectionSavePending,
  useStageSelection,
  type ModelAvailability,
  type ModelRef,
} from '@/entities/model-catalog'
import { ClipService, appFailureFromConnect, type AppFailure } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import {
  clipModelsReady,
  clipQuoteBinding,
  readyClipBatch,
  selectedClipStatus,
  type ClipEligibilityState,
} from '../model/preconditions'

interface Ownership {
  begin(batchId: string): boolean
  owned(batchId: string, jobId: string): void
  rejected(batchId: string): void
}
type StartInput =
  | { kind: 'generate'; batchId: string; quote: ClipQuote; observe: ModelRef; write: ModelRef }
  | { kind: 'render'; batchId: string; revision: number }
const DEFINITE_REFUSALS = new Set([
  'CLIP_QUOTE_REQUIRED',
  'CLIP_QUOTE_CHANGED',
  'CLIP_QUOTE_EXPIRED',
  'CLIP_MODEL_PRICING_UNAVAILABLE',
  'CLIP_MODEL_INPUT_UNSUPPORTED',
  'CLIP_MODEL_VIDEO_INPUT_ABSENT',
  'CLIP_MODEL_INLINE_ENDPOINT_UNAVAILABLE',
  'CLIP_MODEL_REQUIRED_PARAMETERS_UNSUPPORTED',
  'CLIP_MODEL_PRICE_CEILING_UNAVAILABLE',
  'CLIP_SOURCE_UNAVAILABLE',
  'CLIP_NOT_FOUND',
  'CLIP_INVALID_INPUT',
  'CLIP_BUSY',
  'CLIP_PLAN_CONFLICT',
  'MODEL_VIDEO_UNSUPPORTED',
  'MODEL_UNAVAILABLE',
  'PROVIDER_DISABLED',
  'INSUFFICIENT_CREDITS',
])

export function useGenerateClip(ownerId: string, project: ClipProject, ownedJobId?: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const selectionPending = useSelectionSavePending()
  const { t } = useTranslation('clips')
  // T111's live answer for every registered observe model. It is the authority for THIS
  // workflow: the picker greys what it refuses, the action closes while it is loading or
  // failed, and a quote is bound to the status it was read under.
  const eligibilityQuery = useClipAnalysisEligibility(ownerId)
  const eligibility: ClipEligibilityState = eligibilityQuery.isError
    ? { kind: 'failed' }
    : eligibilityQuery.data
      ? { kind: 'ready', rows: eligibilityQuery.data }
      : { kind: 'loading' }
  const observeStatus = selectedClipStatus(eligibility, observe.selected)
  const availability: ModelAvailability =
    eligibility.kind === 'ready'
      ? {
          kind: 'ready',
          resolve: (ref) => {
            const status = selectedClipStatus(eligibility, ref)
            if (status === 'eligible') return { usable: true }
            return {
              usable: false,
              reason: status
                ? t(`generation.eligibility.reason.${status}`)
                : t('generation.eligibility.unresolved'),
            }
          },
        }
      : eligibility.kind === 'failed'
        ? { kind: 'failed', retry: () => void eligibilityQuery.refetch() }
        : { kind: 'loading' }
  const [started, setStarted] = useState<{ id: string; previous?: string }>()
  const [localFailure, setLocalFailure] = useState<AppFailure>()
  const [uncertain, setUncertain] = useState<{
    batchId: string
    quoteId?: string
    ownership: Ownership
  }>()
  const unresolved = useRef<typeof uncertain>(undefined)
  const starting = useRef(false)
  const active = useRef(true)
  const consumed = useRef(new Set<string>())
  const usedQuotes = useRef(new Set<string>())
  useEffect(() => {
    active.current = true
    return () => {
      active.current = false
      unresolved.current = undefined
    }
  }, [])
  const mutation = useMutation({
    mutationFn: async (input: StartInput) => {
      const client = createClient(ClipService, transport)
      const response =
        input.kind === 'render'
          ? await client.startClipRender({
              projectId: project.id,
              batchId: input.batchId,
              expectedRevision: input.revision,
            })
          : await client.startClipGeneration({
              projectId: project.id,
              batchId: input.batchId,
              observeModel: input.observe,
              writeModel: input.write,
              quoteId: input.quote.quoteId,
              approvedMaxCredits: input.quote.maxCredits,
            })
      if (!response.jobId) throw new Error('Missing durable clip job')
      return response
    },
    // Never pause a paid request offline and resume it on reconnect, nor replay it.
    retry: false,
    networkMode: 'always',
    gcTime: 0,
  })
  const latestId =
    started && (!project.latestJob || project.latestJob.id === started.previous)
      ? started.id
      : (project.latestJob?.id ?? started?.id ?? '')
  // Finish this page's accepted media owner even if another tab has already
  // started a newer job by the time the project projection arrives.
  const id = ownedJobId ?? latestId
  const poll = useJob(id, [clipProjectsKey(transport, ownerId), myPlanQueryKey(transport)])
  const job =
    poll.job?.id === id ? poll.job : project.latestJob?.id === id ? project.latestJob : undefined
  const busy =
    mutation.isPending ||
    !!uncertain ||
    (!!id && (!job || !isTerminal(job))) ||
    (!!project.latestJob && project.latestJob.id !== id && !isTerminal(project.latestJob))
  const modelsReady = !selectionPending && clipModelsReady(observe, write, eligibility)
  const resolution = useQuery({
    queryKey: ['clip-start-resolution', transport, ownerId, project.id, uncertain?.batchId],
    enabled: !!uncertain,
    gcTime: 0,
    staleTime: 0,
    refetchInterval: uncertain ? POLL_INTERVAL_MS : false,
    queryFn: async ({ signal }) => {
      const response = await createClient(ClipService, transport).getClipProject(
        { id: project.id },
        { signal },
      )
      if (!response.project) throw new Error('Missing owned clip')
      const found = toClipProject(response.project)
      const attempt = found.latestAttempt
      if (
        active.current &&
        uncertain &&
        unresolved.current === uncertain &&
        attempt &&
        attempt.batchId === uncertain.batchId &&
        attempt.quoteId === (uncertain.quoteId ?? '') &&
        found.latestJob?.id === attempt.jobId
      ) {
        unresolved.current = undefined
        uncertain.ownership.owned(uncertain.batchId, attempt.jobId)
        setStarted({ id: attempt.jobId, previous: project.latestJob?.id })
        setUncertain(undefined)
        setLocalFailure(undefined)
        void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
      }
      return found
    },
  })

  const accounting = project.accounting?.jobId === job?.id ? project.accounting : undefined
  const balanceTransition = accounting
    ? JSON.stringify([
        accounting.jobId,
        accounting.status,
        accounting.reservedCredits,
        accounting.settled,
        accounting.finalChargeCredits,
        accounting.refundCredits,
      ])
    : ''
  useEffect(() => {
    if (balanceTransition) void cache.invalidateQueries({ queryKey: myPlanQueryKey(transport) })
  }, [balanceTransition, cache, transport])

  async function submit(input: StartInput, ownership: Ownership) {
    if (!ownership.begin(input.batchId)) return
    starting.current = true
    consumed.current.add(input.batchId)
    setLocalFailure(undefined)
    try {
      const response = await mutation.mutateAsync(input)
      if (!active.current) return
      ownership.owned(input.batchId, response.jobId)
      setStarted({ id: response.jobId, previous: project.latestJob?.id })
      void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    } catch (error) {
      if (!active.current) return
      const failure = appFailureFromConnect(error)
      setLocalFailure(failure)
      if (DEFINITE_REFUSALS.has(failure.reason)) {
        consumed.current.delete(input.batchId)
        ownership.rejected(input.batchId)
      } else {
        const pending = {
          batchId: input.batchId,
          quoteId: input.kind === 'generate' ? input.quote.quoteId : undefined,
          ownership,
        }
        unresolved.current = pending
        setUncertain(pending)
      }
      void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    } finally {
      starting.current = false
    }
  }
  async function start(
    batch: ReadyClipBatch | undefined,
    settingsReady: boolean,
    quote: ClipQuote,
    ownership: Ownership,
  ) {
    if (
      starting.current ||
      busy ||
      !settingsReady ||
      !modelsReady ||
      !batch ||
      consumed.current.has(batch.id) ||
      usedQuotes.current.has(quote.quoteId) ||
      !observe.selected ||
      !write.selected
    )
      return
    if (!readyClipBatch(batch, project.id, Date.now())) {
      setLocalFailure({ reason: 'CLIP_SOURCE_UNAVAILABLE', params: {} })
      return
    }
    if (
      quote.binding !==
        clipQuoteBinding(project, batch, observe.selected, write.selected, observeStatus) ||
      Date.parse(quote.expiresAt) <= Date.now()
    ) {
      setLocalFailure({ reason: 'CLIP_QUOTE_EXPIRED', params: {} })
      return
    }
    usedQuotes.current.add(quote.quoteId)
    await submit(
      {
        kind: 'generate',
        batchId: batch.id,
        quote,
        observe: observe.selected,
        write: write.selected,
      },
      ownership,
    )
  }
  async function render(batch: ReadyClipBatch | undefined, revision: number, ownership: Ownership) {
    if (
      starting.current ||
      busy ||
      !batch ||
      revision !== project.editPlanRevision ||
      !project.editing ||
      consumed.current.has(batch.id)
    )
      return
    const required = new Set(project.editing.plan.cuts.map((c) => c.fingerprint))
    if (
      !readyClipBatch(batch, project.id, Date.now()) ||
      batch.sources.length !== required.size ||
      batch.sources.some((s) => !required.delete(s.metadata.fingerprint))
    ) {
      setLocalFailure({ reason: 'CLIP_SOURCE_UNAVAILABLE', params: {} })
      return
    }
    await submit({ kind: 'render', batchId: batch.id, revision }, ownership)
  }
  return {
    job,
    busy,
    modelsReady,
    availability,
    eligibility,
    observeStatus,
    accounting,
    observeRef: observe.selected,
    writeRef: write.selected,
    failure: localFailure ?? (job?.status === 'failed' ? job.failure : undefined),
    starting: mutation.isPending,
    uncertain: !!uncertain,
    pollFailed: poll.isError || (!!uncertain && resolution.isError),
    checkAgain: () => {
      poll.refetch()
      if (uncertain) void resolution.refetch()
      void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    },
    start,
    render,
  }
}
