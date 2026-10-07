import { assign, fromPromise, setup, type SnapshotFrom } from 'xstate'
import type {
  CandidatePreparationClient,
  PreparedCandidates,
  TestCount,
  TestModelRef,
  TestSettingKind,
  CandidatePreparationStartInput,
  CandidatePreparationReadInput,
  CandidatePreparationScope,
  PreparedCandidate,
  PreparedCandidateRef,
} from '@/entities/writing-test'
import { PREPARATION_COUNTS, TEST_COUNTS } from '@/entities/writing-test'
import { appFailureFromConnect, appFailureSpecs, type AppFailure } from '@/shared/api'
import { WRITING_TEST_DIRECTION_MAX_CHARS } from '../config'

export interface PreparationDraft {
  kind: TestSettingKind
  count: CandidatePreparationScope['count']
  /** Full tournament format, separate from this exact missing-slot batch. */
  testCount?: TestCount
  slotIndices?: number[]
  retainedRefs?: PreparedCandidateRef[]
  prompt: string
  writeModel?: TestModelRef
}
export interface PreparationCommand extends PreparationDraft {
  writeModel: TestModelRef
  createKey: string
  startKey: string
  sessionId?: string
  expectedRevision?: number
}
export type PendingPreparationCommand =
  | { kind: 'create'; input: CandidatePreparationScope & { requestKey: string } }
  | { kind: 'start'; input: CandidatePreparationStartInput }
  | { kind: 'cancel'; input: CandidatePreparationReadInput & { jobId: string } }
  | { kind: 'read'; input: CandidatePreparationReadInput }
export interface PreparationRecovery {
  ownerId?: string
  draft?: PreparationDraft
  command?: PreparationCommand
  session?: PreparedCandidates
  pending?: PendingPreparationCommand
  estimate?: { free: boolean; credits: number }
  artifacts?: PreparedCandidate[]
}
interface Input {
  ownerId: string
  seedKey: string
  client: CandidatePreparationClient
  draft: PreparationDraft
  recovery?: PreparationRecovery
  requestKey?: () => string
}
export interface PreparationContext {
  scopeKey: string
  ownerId: string
  client: CandidatePreparationClient
  draft: PreparationDraft
  command?: PreparationCommand
  pending?: PendingPreparationCommand
  session?: PreparedCandidates
  /** Only artifacts referenced by this kind's current plan, plus its newly ready batch. */
  artifacts: PreparedCandidate[]
  estimate?: { free: boolean; credits: number }
  failure?: AppFailure
  requestKey: () => string
  suspended: boolean
  operation: number
  uncertain: boolean
  /** Only a live first confirmation may continue its create into start. Recovery never does. */
  continueAfterCreate: boolean
}
export type PreparationEvent = { scopeKey: string } & (
  | { type: 'EDIT'; draft: PreparationDraft }
  | { type: 'RETAIN'; refs: PreparedCandidateRef[] }
  | { type: 'ESTIMATE' | 'CONFIRM' | 'BACK' | 'REFRESH' | 'RETRY' | 'CANCEL' | 'SUSPEND' }
  | { type: 'HYDRATE'; operation: number; session: PreparedCandidates }
)
export type PreparationPhase =
  | 'idle'
  | 'quoting'
  | 'quoted'
  | 'creating'
  | 'created'
  | 'starting'
  | 'reading'
  | 'running'
  | 'cancelling'
  | 'ready'
  | 'failed'
  | 'uncertain'
  | 'suspended'
function record(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}
function revision(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0 && value <= 0xffffffff
}
function scopeShape(value: unknown): value is CandidatePreparationScope {
  return (
    record(value) &&
    typeof value.kind === 'string' &&
    ['writing-voice', 'post-template', 'post-guideline'].includes(value.kind) &&
    typeof value.count === 'number' &&
    PREPARATION_COUNTS.includes(value.count as CandidatePreparationScope['count'])
  )
}
function modelShape(value: unknown): value is TestModelRef {
  return record(value) && typeof value.providerId === 'string' && typeof value.modelId === 'string'
}
function draftShape(value: unknown): value is PreparationDraft {
  return (
    scopeShape(value) &&
    'prompt' in value &&
    typeof value.prompt === 'string' &&
    (!('testCount' in value) ||
      value.testCount === undefined ||
      TEST_COUNTS.includes(value.testCount as TestCount)) &&
    (!('testCount' in value) ||
      value.testCount === undefined ||
      value.count <= Number(value.testCount)) &&
    (!('slotIndices' in value) ||
      value.slotIndices === undefined ||
      (Array.isArray(value.slotIndices) &&
        value.slotIndices.length === value.count &&
        new Set(value.slotIndices).size === value.count &&
        value.slotIndices.every(
          (index) =>
            Number.isInteger(index) &&
            index >= 0 &&
            index < Number(('testCount' in value && value.testCount) || value.count),
        ))) &&
    (!('retainedRefs' in value) ||
      value.retainedRefs === undefined ||
      refsShape(value.retainedRefs)) &&
    (!('writeModel' in value) || value.writeModel === undefined || modelShape(value.writeModel))
  )
}
function refShape(value: unknown): value is PreparedCandidateRef {
  return (
    record(value) &&
    value.type === 'authoring' &&
    record(value.authoring) &&
    typeof value.authoring.sessionId === 'string' &&
    !!value.authoring.sessionId &&
    typeof value.authoring.candidateId === 'string' &&
    !!value.authoring.candidateId &&
    revision(value.authoring.revision) &&
    value.authoring.revision > 0
  )
}
function refKey(value: PreparedCandidateRef): string {
  const ref = value.authoring
  return JSON.stringify([ref.sessionId, ref.candidateId, ref.revision])
}
function refsShape(value: unknown): value is PreparedCandidateRef[] {
  return (
    Array.isArray(value) &&
    value.length <= 16 &&
    value.every(refShape) &&
    new Set(value.map(refKey)).size === value.length
  )
}
function artifactShape(value: unknown): value is PreparedCandidate {
  return (
    record(value) &&
    typeof value.id === 'string' &&
    !!value.id &&
    ['name', 'description', 'body', 'titleArea'].every((key) => typeof value[key] === 'string') &&
    !!String(value.name).trim() &&
    !!String(value.body).trim() &&
    revision(value.revision) &&
    value.revision > 0 &&
    refShape(value.source) &&
    value.source.authoring.candidateId === value.id &&
    value.source.authoring.revision === value.revision
  )
}
function retainArtifacts(
  artifacts: PreparedCandidate[],
  refs: PreparedCandidateRef[] = [],
): PreparedCandidate[] {
  const keys = new Set(refs.map(refKey))
  return artifacts.filter((artifact) => keys.has(refKey(artifact.source)))
}
function mergeArtifacts(
  context: PreparationContext,
  session: PreparedCandidates,
): PreparedCandidate[] {
  const kept = retainArtifacts(context.artifacts, context.draft.retainedRefs)
  if (session.status !== 'ready') return kept
  const merged = new Map(kept.map((artifact) => [refKey(artifact.source), artifact]))
  for (const artifact of session.candidates) merged.set(refKey(artifact.source), artifact)
  return [...merged.values()].slice(-16)
}
function commandShape(value: unknown): value is PreparationCommand {
  if (!draftShape(value)) return false
  const candidate = value as unknown as Record<string, unknown>
  return (
    modelShape(candidate.writeModel) &&
    typeof candidate.createKey === 'string' &&
    !!candidate.createKey &&
    typeof candidate.startKey === 'string' &&
    !!candidate.startKey &&
    (candidate.sessionId === undefined || typeof candidate.sessionId === 'string') &&
    (candidate.expectedRevision === undefined || revision(candidate.expectedRevision))
  )
}
function pendingShape(value: unknown): value is PendingPreparationCommand {
  if (!record(value) || !record(value.input) || !scopeShape(value.input)) return false
  const input = value.input as Record<string, unknown>
  if (value.kind === 'create') return typeof input.requestKey === 'string' && !!input.requestKey
  if (typeof input.sessionId !== 'string' || !input.sessionId) return false
  if (value.kind === 'read') return true
  if (value.kind === 'cancel') return typeof input.jobId === 'string' && !!input.jobId
  return (
    value.kind === 'start' &&
    revision(input.expectedRevision) &&
    typeof input.requestKey === 'string' &&
    !!input.requestKey &&
    typeof input.prompt === 'string' &&
    modelShape(input.writeModel)
  )
}
export function preparedCandidatesShape(value: unknown): value is PreparedCandidates {
  if (!scopeShape(value)) return false
  const session = value as unknown as Record<string, unknown>
  if (
    typeof session.sessionId !== 'string' ||
    !session.sessionId ||
    !revision(session.revision) ||
    typeof session.status !== 'string' ||
    !['idle', 'running', 'ready', 'failed', 'cancelled'].includes(session.status) ||
    typeof session.activeJobId !== 'string' ||
    !Array.isArray(session.candidates) ||
    (session.status === 'running' && !session.activeJobId) ||
    (session.status === 'ready' && session.candidates.length !== value.count) ||
    (session.candidates.length > 0 && session.candidates.length !== value.count) ||
    (session.failure !== undefined &&
      (!record(session.failure) ||
        typeof session.failure.reason !== 'string' ||
        !Object.hasOwn(appFailureSpecs, session.failure.reason) ||
        !record(session.failure.params) ||
        !Object.values(session.failure.params).every((value) => typeof value === 'string')))
  )
    return false
  return (
    new Set(session.candidates.map((candidate) => (record(candidate) ? candidate.id : undefined)))
      .size === session.candidates.length &&
    session.candidates.every(
      (candidate) =>
        artifactShape(candidate) && candidate.source.authoring.sessionId === session.sessionId,
    )
  )
}
function estimateShape(value: unknown): value is { free: boolean; credits: number } {
  return (
    record(value) &&
    typeof value.free === 'boolean' &&
    typeof value.credits === 'number' &&
    Number.isFinite(value.credits) &&
    value.credits >= 0 &&
    (!value.free || value.credits === 0)
  )
}
export function preparationRecovery(value: unknown): PreparationRecovery | undefined {
  if (
    !record(value) ||
    !draftShape(value.draft) ||
    (value.command !== undefined && !commandShape(value.command)) ||
    (value.pending !== undefined && !pendingShape(value.pending)) ||
    (value.session !== undefined && !preparedCandidatesShape(value.session)) ||
    (value.estimate !== undefined && !estimateShape(value.estimate)) ||
    (value.ownerId !== undefined && (typeof value.ownerId !== 'string' || !value.ownerId)) ||
    (value.artifacts !== undefined &&
      (!Array.isArray(value.artifacts) ||
        value.artifacts.length > 16 ||
        (value.artifacts.length > 0 && !value.ownerId) ||
        !value.artifacts.every(artifactShape) ||
        new Set(value.artifacts.map((artifact) => refKey(artifact.source))).size !==
          value.artifacts.length))
  )
    return undefined
  const recovery = value as unknown as PreparationRecovery
  const draft = recovery.command ?? recovery.draft!
  if (
    (recovery.command &&
      (recovery.command.kind !== recovery.draft!.kind ||
        recovery.command.count !== recovery.draft!.count ||
        recovery.command.testCount !== recovery.draft!.testCount ||
        JSON.stringify(recovery.command.slotIndices) !==
          JSON.stringify(recovery.draft!.slotIndices))) ||
    (recovery.session &&
      (recovery.session.kind !== draft.kind ||
        recovery.session.count !== draft.count ||
        (recovery.command?.sessionId &&
          recovery.session.sessionId !== recovery.command.sessionId))) ||
    (recovery.pending &&
      (recovery.pending.input.kind !== draft.kind ||
        recovery.pending.input.count !== draft.count ||
        ('sessionId' in recovery.pending.input &&
          recovery.session &&
          recovery.pending.input.sessionId !== recovery.session.sessionId)))
  )
    return undefined
  const command = recovery.command
  const pending = recovery.pending
  if (pending?.kind === 'create' && (!command || pending.input.requestKey !== command.createKey))
    return undefined
  if (
    pending?.kind === 'start' &&
    (!command ||
      pending.input.requestKey !== command.startKey ||
      pending.input.sessionId !== command.sessionId ||
      pending.input.expectedRevision !== command.expectedRevision ||
      pending.input.prompt !== command.prompt ||
      pending.input.writeModel.providerId !== command.writeModel.providerId ||
      pending.input.writeModel.modelId !== command.writeModel.modelId)
  )
    return undefined
  return structuredClone(recovery)
}
function problem(draft: PreparationDraft): boolean {
  return (
    !draftShape(draft) ||
    !draft.writeModel?.providerId ||
    !draft.writeModel.modelId ||
    !draft.prompt.trim() ||
    Array.from(draft.prompt).length > WRITING_TEST_DIRECTION_MAX_CHARS
  )
}
function failure(error: unknown): AppFailure {
  if (
    record(error) &&
    record(error.failure) &&
    typeof error.failure.reason === 'string' &&
    Object.hasOwn(appFailureSpecs, error.failure.reason)
  )
    return error.failure as unknown as AppFailure
  if (
    record(error) &&
    typeof error.reason === 'string' &&
    Object.hasOwn(appFailureSpecs, error.reason)
  )
    return error as unknown as AppFailure
  return appFailureFromConnect(error)
}
function owns(context: PreparationContext, event: PreparationEvent): boolean {
  return !!context.ownerId && !context.suspended && event.scopeKey === context.scopeKey
}
interface PreparationResult {
  scopeKey: string
  operation: number
  session: PreparedCandidates
}
interface PreparationQuote {
  scopeKey: string
  operation: number
  estimate: { free: boolean; credits: number }
}
function result(event: unknown): PreparationResult | undefined {
  const incoming = event as { output?: PreparationResult } & PreparationEvent
  return incoming.output ?? (incoming.type === 'HYDRATE' ? incoming : undefined)
}
function accepts(context: PreparationContext, event: unknown): boolean {
  const incoming = result(event)
  if (
    !incoming ||
    context.suspended ||
    incoming.scopeKey !== context.scopeKey ||
    incoming.operation !== context.operation ||
    !preparedCandidatesShape(incoming.session)
  )
    return false
  const current = context.command ?? context.draft
  const expectedId = context.session?.sessionId ?? context.command?.sessionId
  return (
    incoming.session.kind === current.kind &&
    incoming.session.count === current.count &&
    (!expectedId || incoming.session.sessionId === expectedId) &&
    (!context.session || incoming.session.revision >= context.session.revision)
  )
}
function readInput(context: PreparationContext): CandidatePreparationReadInput {
  const draft = context.command ?? context.draft
  return {
    kind: draft.kind,
    count: draft.count,
    sessionId: context.session?.sessionId ?? context.command?.sessionId ?? '',
  }
}
function startInput(command: PreparationCommand): CandidatePreparationStartInput {
  return {
    kind: command.kind,
    count: command.count,
    prompt: command.prompt,
    writeModel: { ...command.writeModel },
    requestKey: command.startKey,
    sessionId: command.sessionId!,
    expectedRevision: command.expectedRevision!,
  }
}
function readSettles(context: PreparationContext, session: PreparedCandidates): boolean {
  const pending = context.pending
  if (!pending || pending.kind === 'read' || pending.kind === 'create') return true
  if (pending.kind === 'cancel')
    return (
      session.status === 'cancelled' || session.status === 'ready' || session.status === 'failed'
    )
  return session.status !== 'idle' && session.revision > pending.input.expectedRevision
}
const invokeWork = {
  src: 'execute' as const,
  input: ({ context }: { context: PreparationContext }) => context,
  onDone: [
    { guard: 'accepts' as const, target: 'received', actions: 'received' as const },
    { target: 'uncertain', actions: 'invalidResponse' as const },
  ],
  onError: { target: 'workFailed', actions: 'workErrored' as const },
}
/** Requests and recovery use one statechart; an uncertain cancel can only retry that cancel. */
export const candidatePreparationMachine = setup({
  types: { context: {} as PreparationContext, input: {} as Input, events: {} as PreparationEvent },
  actors: {
    estimate: fromPromise<PreparationQuote, PreparationContext>(async ({ input, signal }) => {
      const estimate = await input.client.estimate(
        { kind: input.draft.kind, count: input.draft.count, writeModel: input.draft.writeModel! },
        signal,
      )
      if (!estimateShape(estimate)) throw new Error('Unconfirmed estimate')
      return { scopeKey: input.scopeKey, operation: input.operation, estimate }
    }),
    execute: fromPromise<PreparationResult, PreparationContext>(async ({ input, signal }) => {
      const pending = input.pending!
      let session: PreparedCandidates
      if (pending.kind === 'create') session = await input.client.create(pending.input, signal)
      else if (pending.kind === 'start') session = await input.client.start(pending.input, signal)
      else if (pending.kind === 'cancel') session = await input.client.cancel(pending.input, signal)
      else session = await input.client.get(pending.input, signal)
      return { scopeKey: input.scopeKey, operation: input.operation, session }
    }),
    read: fromPromise<PreparationResult, PreparationContext>(async ({ input, signal }) => ({
      scopeKey: input.scopeKey,
      operation: input.operation,
      session: await input.client.get(readInput(input), signal),
    })),
  },
  guards: {
    scoped: ({ context, event }) => owns(context, event),
    canEdit: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'EDIT' &&
      draftShape(event.draft) &&
      !context.uncertain &&
      !context.pending,
    canRetain: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'RETAIN' &&
      !context.pending &&
      !context.uncertain &&
      refsShape(event.refs),
    canEstimate: ({ context, event }) =>
      owns(context, event) && !context.uncertain && !context.pending && !problem(context.draft),
    hasSession: ({ context }) => !!(context.session?.sessionId ?? context.command?.sessionId),
    canRefresh: ({ context, event }) =>
      owns(context, event) && !!(context.session?.sessionId ?? context.command?.sessionId),
    isReady: ({ context }) =>
      context.session?.status === 'ready' && preparedCandidatesShape(context.session),
    isRunning: ({ context }) => context.session?.status === 'running',
    isFailed: ({ context }) =>
      context.session?.status === 'failed' || context.session?.status === 'cancelled',
    isUncertain: ({ context }) => context.uncertain,
    canContinue: ({ context }) =>
      context.continueAfterCreate &&
      context.session?.status === 'idle' &&
      !!context.command?.sessionId,
    isCreated: ({ context }) => context.session?.status === 'idle' && !!context.command?.sessionId,
    canConfirm: ({ context, event }) =>
      owns(context, event) &&
      !!context.estimate &&
      estimateShape(context.estimate) &&
      !problem(context.draft),
    canConfirmStart: ({ context, event }) =>
      owns(context, event) &&
      !!context.command?.sessionId &&
      revision(context.command.expectedRevision) &&
      !problem(context.command) &&
      !context.uncertain &&
      !context.pending,
    canCancel: ({ context, event }) =>
      owns(context, event) && !context.pending && !!context.session?.activeJobId,
    accepts: ({ context, event }) => accepts(context, event),
    acceptsQuote: ({ context, event }) => {
      const incoming = (event as unknown as { output?: PreparationQuote }).output
      return (
        !!incoming &&
        !context.suspended &&
        incoming.scopeKey === context.scopeKey &&
        incoming.operation === context.operation &&
        estimateShape(incoming.estimate)
      )
    },
    retryCreate: ({ context, event }) =>
      owns(context, event) && context.uncertain && context.pending?.kind === 'create',
    retryStart: ({ context, event }) =>
      owns(context, event) && context.uncertain && context.pending?.kind === 'start',
    retryCancel: ({ context, event }) =>
      owns(context, event) && context.uncertain && context.pending?.kind === 'cancel',
    retryRead: ({ context, event }) =>
      owns(context, event) && context.uncertain && context.pending?.kind === 'read',
  },
  actions: {
    edit: assign(({ context, event }) =>
      event.type === 'EDIT'
        ? {
            draft: structuredClone(event.draft),
            artifacts:
              event.draft.kind === context.draft.kind
                ? retainArtifacts(context.artifacts, event.draft.retainedRefs)
                : [],
            estimate: undefined,
            failure: undefined,
            command: undefined,
            session: undefined,
            pending: undefined,
            uncertain: false,
          }
        : {},
    ),
    retain: assign(({ context, event }) =>
      event.type === 'RETAIN'
        ? {
            draft: { ...context.draft, retainedRefs: structuredClone(event.refs) },
            artifacts: retainArtifacts(context.artifacts, event.refs),
          }
        : {},
    ),
    quoting: assign(({ context }) => ({
      estimate: undefined,
      failure: undefined,
      operation: context.operation + 1,
    })),
    clearEstimate: assign({ estimate: undefined, failure: undefined }),
    quoted: assign(({ event }) => ({
      estimate: (event as unknown as { output: PreparationQuote }).output.estimate,
    })),
    freeze: assign(({ context }) => {
      const command = {
        ...structuredClone(context.draft),
        writeModel: structuredClone(context.draft.writeModel!),
        createKey: context.requestKey(),
        startKey: context.requestKey(),
      }
      return {
        command,
        pending: {
          kind: 'create' as const,
          input: { kind: command.kind, count: command.count, requestKey: command.createKey },
        },
        uncertain: false,
        continueAfterCreate: true,
        failure: undefined,
        session: undefined,
        operation: context.operation + 1,
      }
    }),
    beginStart: assign(({ context }) => ({
      pending: { kind: 'start' as const, input: startInput(context.command!) },
      uncertain: false,
      continueAfterCreate: false,
      operation: context.operation + 1,
      failure: undefined,
    })),
    beginCancel: assign(({ context }) => ({
      pending: {
        kind: 'cancel' as const,
        input: { ...readInput(context), jobId: context.session!.activeJobId },
      },
      uncertain: false,
      continueAfterCreate: false,
      operation: context.operation + 1,
      failure: undefined,
    })),
    beginRead: assign(({ context }) => ({
      operation: context.operation + 1,
      continueAfterCreate: false,
      failure: undefined,
      pending: context.pending ?? { kind: 'read' as const, input: readInput(context) },
    })),
    retry: assign(({ context }) => ({
      operation: context.operation + 1,
      continueAfterCreate: false,
      failure: undefined,
    })),
    received: assign(({ context, event }) => {
      const session = result(event)!.session
      const wasCreate = context.pending?.kind === 'create'
      return {
        session,
        artifacts: mergeArtifacts(context, session),
        failure: session.failure,
        uncertain: false,
        pending: undefined,
        command: context.command
          ? {
              ...context.command,
              sessionId: session.sessionId,
              expectedRevision: wasCreate ? session.revision : context.command.expectedRevision,
            }
          : undefined,
      }
    }),
    hydrated: assign(({ context, event }) => {
      const session = result(event)!.session
      const settles = readSettles(context, session)
      return {
        session,
        artifacts: mergeArtifacts(context, session),
        failure: session.failure,
        pending: settles ? undefined : context.pending,
        uncertain: !settles,
        command: context.command
          ? {
              ...context.command,
              sessionId: session.sessionId,
              expectedRevision: context.command.expectedRevision ?? session.revision,
            }
          : undefined,
      }
    }),
    errored: assign(({ event }) => ({
      failure: failure((event as unknown as { error: unknown }).error),
      uncertain: true,
      continueAfterCreate: false,
    })),
    workErrored: assign(({ context, event }) => {
      const refused = failure((event as unknown as { error: unknown }).error)
      const uncertain =
        context.pending?.kind === 'cancel' ||
        ['UNKNOWN_FAILURE', 'NETWORK_UNAVAILABLE'].includes(refused.reason)
      return {
        failure: refused,
        uncertain,
        continueAfterCreate: false,
        pending: uncertain ? context.pending : undefined,
        command: uncertain ? context.command : undefined,
      }
    }),
    quoteErrored: assign(({ event }) => ({
      failure: failure((event as unknown as { error: unknown }).error),
    })),
    invalidResponse: assign({
      failure: { reason: 'UNKNOWN_FAILURE', params: {} },
      uncertain: true,
      continueAfterCreate: false,
    }),
    settled: assign({
      command: undefined,
      pending: undefined,
      uncertain: false,
      continueAfterCreate: false,
    }),
    suspend: assign(({ context }) => ({
      suspended: true,
      artifacts: [],
      operation: context.operation + 1,
      continueAfterCreate: false,
    })),
  },
}).createMachine({
  id: 'candidatePreparation',
  initial: 'restore',
  context: ({ input }) => {
    const restored = preparationRecovery(input.recovery)
    const recovery =
      (restored?.ownerId && restored.ownerId !== input.ownerId) ||
      (restored?.draft && restored.draft.kind !== input.draft.kind)
        ? undefined
        : restored
    const command = recovery?.command
    const pending =
      recovery?.pending ??
      (command
        ? command.sessionId
          ? { kind: 'start' as const, input: startInput(command) }
          : {
              kind: 'create' as const,
              input: { kind: command.kind, count: command.count, requestKey: command.createKey },
            }
        : undefined)
    return {
      scopeKey: JSON.stringify([input.ownerId, input.seedKey]),
      ownerId: input.ownerId,
      client: input.client,
      draft: recovery?.draft ?? structuredClone(input.draft),
      command,
      pending,
      session: recovery?.session,
      artifacts:
        recovery?.draft?.kind === input.draft.kind
          ? retainArtifacts(recovery.artifacts ?? [], input.draft.retainedRefs)
          : [],
      estimate: recovery?.estimate,
      requestKey: input.requestKey ?? (() => crypto.randomUUID()),
      suspended: false,
      operation: 0,
      uncertain: !!pending,
      continueAfterCreate: false,
    }
  },
  on: {
    SUSPEND: { guard: 'scoped', target: '.suspended', actions: 'suspend' },
  },
  states: {
    restore: {
      always: [
        { guard: 'hasSession', target: 'reading', actions: 'beginRead' },
        { guard: 'isUncertain', target: 'uncertain' },
        { target: 'idle' },
      ],
    },
    idle: {
      on: {
        RETAIN: { guard: 'canRetain', actions: 'retain' },
        EDIT: { guard: 'canEdit', actions: 'edit' },
        ESTIMATE: { guard: 'canEstimate', target: 'quoting', actions: 'quoting' },
      },
    },
    quoting: {
      invoke: {
        src: 'estimate',
        input: ({ context }) => context,
        onDone: [
          { guard: 'acceptsQuote', target: 'quoted', actions: 'quoted' },
          { target: 'failed', actions: 'quoteErrored' },
        ],
        onError: { target: 'failed', actions: 'quoteErrored' },
      },
    },
    quoted: {
      on: {
        RETAIN: { guard: 'canRetain', target: 'idle', actions: ['retain', 'clearEstimate'] },
        CONFIRM: { guard: 'canConfirm', target: 'creating', actions: 'freeze' },
        BACK: { guard: 'scoped', target: 'idle', actions: 'clearEstimate' },
        EDIT: { guard: 'canEdit', target: 'idle', actions: 'edit' },
      },
    },
    creating: { invoke: invokeWork },
    workFailed: { always: [{ guard: 'isUncertain', target: 'uncertain' }, { target: 'failed' }] },
    created: {
      on: {
        CONFIRM: { guard: 'canConfirmStart', target: 'starting', actions: 'beginStart' },
        REFRESH: { guard: 'canRefresh', target: 'reading', actions: 'beginRead' },
      },
    },
    starting: { invoke: invokeWork },
    received: {
      always: [
        { guard: 'isUncertain', target: 'uncertain' },
        { guard: 'isReady', target: 'ready', actions: 'settled' },
        { guard: 'isRunning', target: 'running' },
        { guard: 'isFailed', target: 'failed', actions: 'settled' },
        { guard: 'canContinue', target: 'starting', actions: 'beginStart' },
        { guard: 'isCreated', target: 'created' },
        { target: 'idle' },
      ],
    },
    reading: {
      invoke: {
        src: 'read',
        input: ({ context }) => context,
        onDone: [
          { guard: 'accepts', target: 'received', actions: 'hydrated' },
          { target: 'uncertain', actions: 'invalidResponse' },
        ],
        onError: { target: 'uncertain', actions: 'errored' },
      },
      on: { HYDRATE: { guard: 'accepts', target: 'received', actions: 'hydrated' } },
    },
    running: {
      on: {
        REFRESH: { guard: 'canRefresh', target: 'reading', actions: 'beginRead' },
        CANCEL: { guard: 'canCancel', target: 'cancelling', actions: 'beginCancel' },
      },
    },
    cancelling: { invoke: invokeWork },
    ready: {
      on: {
        RETAIN: { guard: 'canRetain', actions: 'retain' },
        EDIT: { guard: 'canEdit', target: 'idle', actions: 'edit' },
        REFRESH: { guard: 'canRefresh', target: 'reading', actions: 'beginRead' },
      },
    },
    failed: {
      on: {
        RETAIN: { guard: 'canRetain', actions: 'retain' },
        EDIT: { guard: 'canEdit', target: 'idle', actions: 'edit' },
        ESTIMATE: { guard: 'canEstimate', target: 'quoting', actions: 'quoting' },
        REFRESH: { guard: 'canRefresh', target: 'reading', actions: 'beginRead' },
      },
    },
    uncertain: {
      on: {
        REFRESH: { guard: 'canRefresh', target: 'reading', actions: 'beginRead' },
        RETRY: [
          { guard: 'retryCreate', target: 'creating', actions: 'retry' },
          { guard: 'retryStart', target: 'starting', actions: 'retry' },
          { guard: 'retryCancel', target: 'cancelling', actions: 'retry' },
          { guard: 'retryRead', target: 'reading', actions: 'beginRead' },
        ],
      },
    },
    suspended: {},
  },
})
export type PreparationSnapshot = SnapshotFrom<typeof candidatePreparationMachine>
