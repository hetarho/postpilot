import { assign, fromPromise, setup, type SnapshotFrom } from 'xstate'
import type {
  WritingTest,
  WritingTestClient,
  WritingTestPlan,
  WritingTestQuote,
  WritingTestPublication,
  WritingTestMatch,
  TestStartInput,
  TestRetryInput,
  TestRetryEstimateInput,
  TestDecisionInput,
  TestOperationInput,
  TestWinnerInput,
  TestApplyInput,
} from '@/entities/writing-test'
import {
  POST_TARGET_LENGTH_MIN,
  POST_TARGET_LENGTH_MAX,
  POST_TAG_COUNT_MIN,
  POST_TAG_COUNT_MAX,
} from '@/entities/post'
import { appFailureFromConnect, appFailureSpecs, type AppFailure } from '@/shared/api'

export type FrozenTestCommand =
  | { kind: 'start'; input: TestStartInput }
  | { kind: 'retry'; input: TestRetryInput }
  | { kind: 'vote'; input: TestDecisionInput }
  | { kind: 'cancel'; input: TestOperationInput }
  | { kind: 'publish'; input: TestWinnerInput }
  | { kind: 'apply'; input: TestApplyInput }

export interface WritingTestRecovery {
  draft?: WritingTestPlan
  testId?: string
  command?: FrozenTestCommand
}
export interface WritingTestMachineInput {
  ownerId: string
  seedKey: string
  client: WritingTestClient
  initialDraft?: WritingTestPlan
  testId?: string
  recovery?: WritingTestRecovery
  now?: () => number
  requestKey?: () => string
}
export interface WritingTestOperationContext {
  scopeKey: string
  ownerId: string
  seedKey: string
  client: WritingTestClient
  now: () => number
  requestKey: () => string
  draft?: WritingTestPlan
  testId?: string
  test?: WritingTest
  operation: number
  quote?: WritingTestQuote
  quotePlan?: WritingTestPlan
  retryEstimate?: TestRetryEstimateInput
  command?: FrozenTestCommand
  publication?: WritingTestPublication
  failure?: AppFailure
  /** A read cannot settle an unknown mutation merely because it returned a record. */
  uncertain: boolean
  suspended: boolean
}
type Scoped = { scopeKey: string }
export type WritingTestOperationEvent = Scoped &
  (
    | { type: 'DRAFT'; draft: WritingTestPlan }
    | { type: 'ESTIMATE' | 'CONFIRM' | 'DISMISS_QUOTE' | 'RELOAD' | 'RETRY_OPERATION' | 'SUSPEND' }
    | { type: 'ESTIMATE_RETRY'; testId: string; expectedRevision: number; candidateIds: string[] }
    | {
        type: 'VOTE'
        testId: string
        expectedRevision: number
        matchId: string
        winnerCandidateId: string
      }
    | { type: 'CANCEL'; testId: string; expectedRevision: number }
    | { type: 'PUBLISH'; input: Omit<TestWinnerInput, 'requestKey'> }
    | { type: 'APPLY'; input: Omit<TestApplyInput, 'requestKey'> }
    | { type: 'HYDRATE'; operation: number; test: WritingTest }
    | { type: 'RESULT'; operation: number; test: WritingTest; publication?: WritingTestPublication }
    | { type: 'QUOTED'; operation: number; quote: WritingTestQuote }
  )
export type WritingTestPhase =
  | 'editing'
  | 'loading'
  | 'quoting'
  | 'quoted'
  | 'quoteExpired'
  | 'starting'
  | 'running'
  | 'partial'
  | 'match'
  | 'deciding'
  | 'retrying'
  | 'champion'
  | 'cancelling'
  | 'cancelled'
  | 'expired'
  | 'publishing'
  | 'published'
  | 'conflict'
  | 'uncertain'
  | 'uncertainPublication'
  | 'failed'
  | 'suspended'

export function writingTestScopeKey(ownerId: string, seedKey: string): string {
  return JSON.stringify([ownerId, seedKey])
}
export function currentWritingTestMatch(test?: WritingTest): WritingTestMatch | undefined {
  if (!test || test.status !== 'review' || !allOutputsSucceeded(test)) return undefined
  return [...test.matches]
    .sort((a, b) => a.round - b.round || a.index - b.index)
    .find(
      (match) =>
        !match.winnerCandidateId &&
        !!match.leftCandidateId &&
        !!match.rightCandidateId &&
        test.candidates.some((c) => c.id === match.leftCandidateId) &&
        test.candidates.some((c) => c.id === match.rightCandidateId),
    )
}
export function allOutputsSucceeded(test: WritingTest): boolean {
  return (
    test.candidates.length === test.count &&
    test.candidates.every((candidate) => candidate.status === 'succeeded' && !!candidate.output)
  )
}
function owned(context: WritingTestOperationContext, event: WritingTestOperationEvent): boolean {
  return !!context.ownerId && !context.suspended && event.scopeKey === context.scopeKey
}
function revision(
  context: WritingTestOperationContext,
  testId: string,
  expectedRevision: number,
): boolean {
  return !!context.test && testId === context.test.id && expectedRevision === context.test.revision
}
function expired(test: WritingTest | undefined, now: number): boolean {
  return (
    !!test &&
    ((!!test.contentExpiresAt && Date.parse(test.contentExpiresAt) <= now) ||
      test.candidates.some((candidate) => candidate.status === 'succeeded' && !candidate.output))
  )
}
function confirmedChampion(test: WritingTest | undefined): test is WritingTest {
  return (
    !!test &&
    test.status === 'completed' &&
    !!test.winnerCandidateId &&
    allOutputsSucceeded(test) &&
    test.matches.filter((match) => !!match.winnerCandidateId).length === test.count - 1 &&
    test.candidates.some((candidate) => candidate.id === test.winnerCandidateId)
  )
}
function candidateKey(entrant: WritingTestPlan['entrants'][number]): string {
  if (entrant.type === 'model')
    return JSON.stringify(['model', entrant.model.providerId, entrant.model.modelId])
  if (entrant.type === 'setting')
    return JSON.stringify(['setting', entrant.setting.kind, entrant.setting.id])
  return JSON.stringify(['authoring', entrant.authoring.sessionId, entrant.authoring.candidateId])
}
function concrete(value: string): boolean {
  return !!value.trim()
}
function publishedAction(
  context: WritingTestOperationContext,
  action: WritingTestPublication['action'],
): boolean {
  return (
    (context.publication?.action === action && context.publication.status === 'confirmed') ||
    !!context.test?.publications.some(
      (receipt) =>
        receipt.action === action &&
        receipt.status === 'confirmed' &&
        receipt.testId === context.test?.id &&
        receipt.winnerCandidateId === context.test.winnerCandidateId,
    )
  )
}
export function writingTestDraftProblem(plan?: WritingTestPlan): AppFailure | undefined {
  const failure = (reason: AppFailure['reason']): AppFailure => ({ reason, params: {} })
  if (
    !plan ||
    ![2, 4, 8, 16].includes(plan.count) ||
    !Array.isArray(plan.entrants) ||
    plan.entrants.length !== plan.count
  )
    return failure('WRITING_TEST_COUNT_INVALID')
  if (!['model', 'voice', 'template', 'guideline'].includes(plan.factor))
    return failure('WRITING_TEST_FACTOR_INVALID')
  if (record(plan.context)) {
    if (
      !Number.isInteger(plan.context.targetLength) ||
      plan.context.targetLength < POST_TARGET_LENGTH_MIN ||
      plan.context.targetLength > POST_TARGET_LENGTH_MAX
    )
      return {
        reason: 'POST_TARGET_LENGTH_INVALID',
        params: { min: String(POST_TARGET_LENGTH_MIN), max: String(POST_TARGET_LENGTH_MAX) },
      }
    if (
      !Number.isInteger(plan.context.tagCount) ||
      plan.context.tagCount < POST_TAG_COUNT_MIN ||
      plan.context.tagCount > POST_TAG_COUNT_MAX
    )
      return failure('POST_TAG_COUNT_INVALID')
  }
  if (!writingTestPlanShape(plan)) return failure('WRITING_TEST_MATERIAL_INVALID')
  if (
    plan.entrants.some((entrant) => {
      if (entrant.type === 'model')
        return (
          plan.factor !== 'model' ||
          !concrete(entrant.model.providerId) ||
          !concrete(entrant.model.modelId)
        )
      if (plan.factor === 'model') return true
      if (entrant.type === 'authoring')
        return (
          !concrete(entrant.authoring.sessionId) ||
          !concrete(entrant.authoring.candidateId) ||
          !Number.isInteger(entrant.authoring.revision) ||
          entrant.authoring.revision < 1 ||
          entrant.authoring.revision > 0xffffffff
        )
      const kind = {
        voice: 'writing-voice',
        template: 'post-template',
        guideline: 'post-guideline',
      }[plan.factor]
      return (
        entrant.setting.kind !== kind ||
        !concrete(entrant.setting.id) ||
        !concrete(entrant.setting.revision)
      )
    })
  )
    return failure('WRITING_TEST_ENTRANT_INVALID')
  if (new Set(plan.entrants.map(candidateKey)).size !== plan.count)
    return failure('WRITING_TEST_ENTRANTS_DUPLICATE')
  if (
    Array.from(plan.context.material.attachmentIds).some(
      (id) => typeof id !== 'string' || !concrete(id),
    ) ||
    new Set(plan.context.material.attachmentIds).size !== plan.context.material.attachmentIds.length
  )
    return failure('WRITING_TEST_MATERIAL_INVALID')
  if (!plan.context.material.text.trim() && !plan.context.material.attachmentIds.length)
    return failure('WRITING_TEST_MATERIAL_INVALID')
  if (
    plan.factor === 'model' &&
    plan.modelStage === 'observe' &&
    !plan.context.material.attachmentIds.length
  )
    return failure('WRITING_TEST_MATERIAL_INVALID')
  if (
    !(plan.factor === 'model' && plan.modelStage === 'write') &&
    (!plan.context.writeModel ||
      !concrete(plan.context.writeModel.providerId) ||
      !concrete(plan.context.writeModel.modelId))
  )
    return failure('WRITING_TEST_ENTRANT_INVALID')
  if (
    plan.context.material.attachmentIds.length &&
    !(plan.factor === 'model' && plan.modelStage === 'observe') &&
    (!plan.context.observeModel ||
      !concrete(plan.context.observeModel.providerId) ||
      !concrete(plan.context.observeModel.modelId))
  )
    return failure('WRITING_TEST_ENTRANT_INVALID')
  if (plan.factor === 'guideline' && !concrete(plan.context.guidelineSlotId))
    return failure('WRITING_TEST_FACTOR_INVALID')
  return undefined
}
function record(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}
function modelShape(value: unknown): boolean {
  return record(value) && typeof value.providerId === 'string' && typeof value.modelId === 'string'
}
/** Browser recovery is untrusted; structural checks precede every use of nested values. */
export function writingTestPlanShape(value: unknown): value is WritingTestPlan {
  if (
    !record(value) ||
    typeof value.factor !== 'string' ||
    !['model', 'voice', 'template', 'guideline'].includes(value.factor) ||
    typeof value.modelStage !== 'string' ||
    !['observe', 'write'].includes(value.modelStage) ||
    typeof value.count !== 'number' ||
    ![2, 4, 8, 16].includes(value.count) ||
    !Array.isArray(value.entrants) ||
    Array.from(value.entrants).some((entrant) => !record(entrant)) ||
    !record(value.context)
  )
    return false
  const context = value.context
  if (
    !record(context.material) ||
    typeof context.material.text !== 'string' ||
    typeof context.material.fictional !== 'boolean' ||
    !Array.isArray(context.material.attachmentIds) ||
    !context.material.attachmentIds.every((id) => typeof id === 'string') ||
    !Array.isArray(context.material.templateAnswers) ||
    !context.material.templateAnswers.every(
      (answer) =>
        record(answer) &&
        typeof answer.label === 'string' &&
        typeof answer.text === 'string' &&
        typeof answer.enabled === 'boolean',
    ) ||
    typeof context.expectedInputRevision !== 'bigint' ||
    context.expectedInputRevision < 0n ||
    context.expectedInputRevision > 9223372036854775807n ||
    typeof context.expectedContentRevision !== 'bigint' ||
    context.expectedContentRevision < 0n ||
    context.expectedContentRevision > 9223372036854775807n ||
    typeof context.targetLanguage !== 'string' ||
    !['ko', 'en'].includes(context.targetLanguage) ||
    !Number.isInteger(context.targetLength) ||
    !Number.isInteger(context.tagCount) ||
    typeof context.useMemory !== 'boolean' ||
    !Array.isArray(context.qualityRules) ||
    !context.qualityRules.every((rule) =>
      ['title-saturation', 'cross-post-phrases', 'in-post-repetition', 'composition'].includes(
        String(rule),
      ),
    ) ||
    !['sourcePostSlug', 'voiceId', 'templateId', 'guidelineSlotId'].every(
      (name) => typeof context[name] === 'string',
    ) ||
    (context.writeModel !== undefined && !modelShape(context.writeModel)) ||
    (context.observeModel !== undefined && !modelShape(context.observeModel))
  )
    return false
  return value.entrants.every((entrant) => {
    if (!record(entrant)) return false
    if (entrant.type === 'model') return modelShape(entrant.model)
    if (entrant.type === 'setting')
      return (
        record(entrant.setting) &&
        ['writing-voice', 'post-template', 'post-guideline'].includes(
          String(entrant.setting.kind),
        ) &&
        typeof entrant.setting.id === 'string' &&
        typeof entrant.setting.revision === 'string'
      )
    return (
      entrant.type === 'authoring' &&
      record(entrant.authoring) &&
      typeof entrant.authoring.sessionId === 'string' &&
      typeof entrant.authoring.candidateId === 'string' &&
      Number.isInteger(entrant.authoring.revision)
    )
  })
}
export function writingTestCommandShape(value: unknown): value is FrozenTestCommand {
  if (
    !record(value) ||
    !record(value.input) ||
    typeof value.input.requestKey !== 'string' ||
    !value.input.requestKey
  )
    return false
  const input = value.input
  if (value.kind === 'start')
    return (
      writingTestPlanShape(input.plan) && typeof input.quoteKey === 'string' && !!input.quoteKey
    )
  if (
    typeof input.testId !== 'string' ||
    !input.testId ||
    !Number.isInteger(input.expectedRevision) ||
    Number(input.expectedRevision) < 0
  )
    return false
  if (value.kind === 'cancel') return true
  if (value.kind === 'vote')
    return typeof input.matchId === 'string' && typeof input.winnerCandidateId === 'string'
  if (value.kind === 'retry')
    return (
      Array.isArray(input.candidateIds) &&
      input.candidateIds.every((id) => typeof id === 'string') &&
      typeof input.quoteKey === 'string'
    )
  if (value.kind === 'apply')
    return (
      typeof input.winnerCandidateId === 'string' &&
      typeof input.expectedInputRevision === 'bigint' &&
      typeof input.expectedContentRevision === 'bigint'
    )
  return (
    value.kind === 'publish' &&
    typeof input.winnerCandidateId === 'string' &&
    ['save-setting', 'use-setting', 'adopt-model'].includes(String(input.action)) &&
    typeof input.makeDefault === 'boolean' &&
    typeof input.name === 'string' &&
    typeof input.scope === 'string' &&
    Array.isArray(input.scopeIds) &&
    input.scopeIds.every((id) => typeof id === 'string')
  )
}
function validQuote(quote?: WritingTestQuote, now = Date.now()): boolean {
  return (
    !!quote &&
    !!quote.quoteKey &&
    Number.isFinite(quote.credits) &&
    quote.credits >= 0 &&
    Number.isFinite(Date.parse(quote.expiresAt)) &&
    Date.parse(quote.expiresAt) > now
  )
}
function failureOf(error: unknown): AppFailure {
  if (
    typeof error === 'object' &&
    error !== null &&
    'reason' in error &&
    typeof error.reason === 'string' &&
    Object.hasOwn(appFailureSpecs, error.reason)
  )
    return error as AppFailure
  return appFailureFromConnect(error)
}
function isUncertain(failure?: AppFailure): boolean {
  return failure?.reason === 'UNKNOWN_FAILURE' || failure?.reason === 'NETWORK_UNAVAILABLE'
}
function isConflict(failure?: AppFailure): boolean {
  return (
    !!failure &&
    [
      'WRITING_TEST_REVISION_CONFLICT',
      'WRITING_TEST_DECISION_CONFLICT',
      'WRITING_TEST_PUBLICATION_CONFLICT',
      'WRITING_TEST_OUTPUT_INCOMPATIBLE',
    ].includes(failure.reason)
  )
}
function publicationCommand(command?: FrozenTestCommand): boolean {
  return command?.kind === 'publish' || command?.kind === 'apply'
}
interface ReadInput {
  client: WritingTestClient
  scopeKey: string
  operation: number
  testId: string
}
interface WorkInput {
  client: WritingTestClient
  scopeKey: string
  operation: number
  command: FrozenTestCommand
}
interface QuoteInput {
  client: WritingTestClient
  scopeKey: string
  operation: number
  plan?: WritingTestPlan
  retry?: TestRetryEstimateInput
}
interface Result {
  scopeKey: string
  operation: number
  test: WritingTest
  publication?: WritingTestPublication
}
interface QuoteResult {
  scopeKey: string
  operation: number
  quote: WritingTestQuote
}
function resultOf(event: unknown): Result | undefined {
  const incoming = event as { output?: Result } & WritingTestOperationEvent
  return (
    incoming.output ??
    (incoming.type === 'RESULT' || incoming.type === 'HYDRATE' ? incoming : undefined)
  )
}
function quoteOf(event: unknown): QuoteResult | undefined {
  const incoming = event as { output?: QuoteResult } & WritingTestOperationEvent
  return incoming.output ?? (incoming.type === 'QUOTED' ? incoming : undefined)
}
function acceptsResult(context: WritingTestOperationContext, event: unknown): boolean {
  const result = resultOf(event)
  if (
    !result ||
    context.suspended ||
    result.scopeKey !== context.scopeKey ||
    result.operation !== context.operation
  )
    return false
  const expected = context.test?.id ?? context.testId
  const command = context.command
  if (
    result.publication &&
    command &&
    (command.kind === 'publish' || command.kind === 'apply') &&
    (result.publication.testId !== command.input.testId ||
      result.publication.requestKey !== command.input.requestKey ||
      result.publication.winnerCandidateId !== command.input.winnerCandidateId ||
      result.publication.action !==
        (command.kind === 'apply' ? 'apply-output' : command.input.action))
  )
    return false
  return (
    (!expected || result.test.id === expected) &&
    (!context.test || result.test.revision >= context.test.revision)
  )
}
function settledCommand(command: FrozenTestCommand | undefined, test: WritingTest): boolean {
  if (!command) return true
  if (command.kind === 'start') return true
  if (command.input.testId !== test.id) return false
  if (command.kind === 'publish' || command.kind === 'apply')
    return test.publications.some(
      (p) => p.requestKey === command.input.requestKey && p.status !== 'pending',
    )
  if (command.kind === 'cancel') return test.status === 'cancelled'
  if (command.kind === 'vote')
    return test.matches.some((m) => m.id === command.input.matchId && !!m.winnerCandidateId)
  return test.revision > command.input.expectedRevision
}
function matchingPublication(
  command: FrozenTestCommand | undefined,
  test: WritingTest,
): WritingTestPublication | undefined {
  return command && publicationCommand(command)
    ? test.publications.find((p) => p.requestKey === command.input.requestKey)
    : undefined
}
function hydrationConflict(command: FrozenTestCommand | undefined, test: WritingTest): boolean {
  if (command?.kind === 'vote') {
    const match = test.matches.find((m) => m.id === command.input.matchId)
    return !!match?.winnerCandidateId && match.winnerCandidateId !== command.input.winnerCandidateId
  }
  return matchingPublication(command, test)?.status === 'conflict'
}
const read = {
  src: 'read' as const,
  input: ({ context }: { context: WritingTestOperationContext }): ReadInput => ({
    client: context.client,
    scopeKey: context.scopeKey,
    operation: context.operation,
    testId: context.testId!,
  }),
  onDone: [
    { guard: 'validResult' as const, target: 'settling', actions: 'hydrate' as const },
    { target: 'readFailure', actions: 'invalidRead' as const },
  ],
  onError: { target: 'readFailure', actions: 'readFailed' as const },
}
const work = {
  src: 'execute' as const,
  input: ({ context }: { context: WritingTestOperationContext }): WorkInput => ({
    client: context.client,
    scopeKey: context.scopeKey,
    operation: context.operation,
    command: context.command!,
  }),
  onDone: [
    { guard: 'validResult' as const, target: 'settling', actions: 'response' as const },
    { target: 'workFailure', actions: 'invalidResponse' as const },
  ],
  onError: { target: 'workFailure', actions: 'workFailed' as const },
}
const reload = { guard: 'canReload' as const, target: 'loading', actions: 'reload' as const }

/** Owns every request and durable fact. A resumed actor reads, never replays a mutation. */
export const writingTestMachine = setup({
  types: {
    context: {} as WritingTestOperationContext,
    input: {} as WritingTestMachineInput,
    events: {} as WritingTestOperationEvent,
  },
  actors: {
    read: fromPromise<Result, ReadInput>(async ({ input, signal }) => ({
      ...input,
      test: await input.client.get(input.testId, signal),
    })),
    quote: fromPromise<QuoteResult, QuoteInput>(async ({ input, signal }) => ({
      scopeKey: input.scopeKey,
      operation: input.operation,
      quote: input.retry
        ? await input.client.estimateRetry(input.retry, signal)
        : await input.client.estimate(input.plan!, signal),
    })),
    execute: fromPromise<Result, WorkInput>(async ({ input, signal }) => {
      const { client, command } = input
      let test: WritingTest
      let publication: WritingTestPublication | undefined
      if (command.kind === 'start') test = await client.start(command.input, signal)
      else if (command.kind === 'retry') test = await client.retry(command.input, signal)
      else if (command.kind === 'vote') test = await client.decide(command.input, signal)
      else if (command.kind === 'cancel') test = await client.cancel(command.input, signal)
      else {
        const result =
          command.kind === 'publish'
            ? await client.saveWinner(command.input, signal)
            : await client.applyOutput(command.input, signal)
        test = result.test
        publication = result.publication
      }
      return { scopeKey: input.scopeKey, operation: input.operation, test, publication }
    }),
  },
  guards: {
    owned: ({ context, event }) => owned(context, event),
    canDraft: ({ context, event }) => owned(context, event) && !context.test && !context.command,
    canEstimate: ({ context, event }) =>
      owned(context, event) &&
      !context.test &&
      !context.command &&
      !writingTestDraftProblem(context.draft),
    canRetryEstimate: ({ context, event }) => {
      if (
        !owned(context, event) ||
        event.type !== 'ESTIMATE_RETRY' ||
        context.command ||
        !revision(context, event.testId, event.expectedRevision) ||
        expired(context.test, context.now()) ||
        !['partial', 'failed'].includes(context.test!.status) ||
        !event.candidateIds.length ||
        new Set(event.candidateIds).size !== event.candidateIds.length
      )
        return false
      return event.candidateIds.every((id) =>
        context.test!.candidates.some((c) => c.id === id && c.status === 'failed'),
      )
    },
    canConfirm: ({ context, event }) =>
      owned(context, event) && validQuote(context.quote, context.now()),
    expiredQuote: ({ context, event }) =>
      owned(context, event) && !validQuote(context.quote, context.now()),
    canVote: ({ context, event }) => {
      if (
        !owned(context, event) ||
        event.type !== 'VOTE' ||
        context.command ||
        !revision(context, event.testId, event.expectedRevision) ||
        expired(context.test, context.now())
      )
        return false
      const match = currentWritingTestMatch(context.test)
      return (
        !!match &&
        match.id === event.matchId &&
        [match.leftCandidateId, match.rightCandidateId].includes(event.winnerCandidateId)
      )
    },
    canCancel: ({ context, event }) =>
      owned(context, event) &&
      event.type === 'CANCEL' &&
      !context.command &&
      revision(context, event.testId, event.expectedRevision) &&
      !['completed', 'cancelled'].includes(context.test!.status),
    canPublish: ({ context, event }) => {
      if (
        !owned(context, event) ||
        event.type !== 'PUBLISH' ||
        context.command ||
        !revision(context, event.input.testId, event.input.expectedRevision) ||
        !confirmedChampion(context.test) ||
        expired(context.test, context.now()) ||
        publishedAction(context, event.input.action) ||
        !allOutputsSucceeded(context.test) ||
        !context.test.winnerCandidateId ||
        event.input.winnerCandidateId !== context.test.winnerCandidateId
      )
        return false
      if (event.input.action === 'adopt-model') return context.test.factor === 'model'
      if (context.test.factor === 'model') return false
      if (
        event.input.action === 'save-setting' &&
        context.test.factor !== 'guideline' &&
        !event.input.name.trim()
      )
        return false
      if (
        context.test.factor === 'voice' &&
        event.input.action === 'save-setting' &&
        context.test.candidates.find(
          (candidate) => candidate.id === context.test?.winnerCandidateId,
        )?.identity?.synthetic !== true
      )
        return false
      if (context.test.factor !== 'guideline') return true
      if (!['global', 'templates', 'fields'].includes(event.input.scope)) return false
      return event.input.scope === 'global'
        ? event.input.scopeIds.length === 0
        : event.input.scopeIds.length > 0 && event.input.scopeIds.every(concrete)
    },
    canApply: ({ context, event }) =>
      owned(context, event) &&
      event.type === 'APPLY' &&
      !context.command &&
      revision(context, event.input.testId, event.input.expectedRevision) &&
      confirmedChampion(context.test) &&
      context.test.factor === 'model' &&
      !!context.test.sourcePostSlug &&
      !expired(context.test, context.now()) &&
      allOutputsSucceeded(context.test) &&
      !publishedAction(context, 'apply-output') &&
      event.input.winnerCandidateId === context.test.winnerCandidateId,
    canReload: ({ context, event }) => owned(context, event) && !!context.testId,
    validResult: ({ context, event }) => acceptsResult(context, event),
    validQuote: ({ context, event }) => {
      const result = quoteOf(event)
      return (
        !!result &&
        !context.suspended &&
        result.scopeKey === context.scopeKey &&
        result.operation === context.operation
      )
    },
    retryStart: ({ context, event }) =>
      owned(context, event) && context.uncertain && context.command?.kind === 'start',
    retryVote: ({ context, event }) =>
      owned(context, event) && context.uncertain && context.command?.kind === 'vote',
    retryCandidates: ({ context, event }) =>
      owned(context, event) && context.uncertain && context.command?.kind === 'retry',
    retryCancel: ({ context, event }) =>
      owned(context, event) && context.uncertain && context.command?.kind === 'cancel',
    retryPublish: ({ context, event }) =>
      owned(context, event) && context.uncertain && publicationCommand(context.command),
    knownTest: ({ context }) => !!context.testId,
    uncertainPublication: ({ context }) => context.uncertain && publicationCommand(context.command),
    uncertain: ({ context }) => context.uncertain,
    conflict: ({ context }) =>
      isConflict(context.failure) || context.publication?.status === 'conflict',
    quoteRequired: ({ context }) => context.failure?.reason === 'WRITING_TEST_QUOTE_REQUIRED',
    hasFailure: ({ context }) => !!context.failure,
    expired: ({ context }) => expired(context.test, context.now()),
    running: ({ context }) =>
      context.test?.status === 'queued' || context.test?.status === 'running',
    partial: ({ context }) =>
      context.test?.status === 'partial' || context.test?.status === 'failed',
    match: ({ context }) => !!currentWritingTestMatch(context.test),
    champion: ({ context }) => confirmedChampion(context.test),
    cancelled: ({ context }) => context.test?.status === 'cancelled',
    published: ({ context }) => context.publication?.status === 'confirmed',
    hasTest: ({ context }) => !!context.test,
  },
  actions: {
    draft: assign(({ event }) =>
      event.type === 'DRAFT'
        ? { draft: structuredClone(event.draft), failure: undefined, quote: undefined }
        : {},
    ),
    invalidDraft: assign(({ context }) => ({ failure: writingTestDraftProblem(context.draft) })),
    freezeEstimate: assign(({ context }) => ({
      quotePlan: structuredClone(context.draft!),
      retryEstimate: undefined,
      quote: undefined,
      failure: undefined,
      operation: context.operation + 1,
    })),
    freezeRetryEstimate: assign(({ context, event }) =>
      event.type === 'ESTIMATE_RETRY'
        ? {
            retryEstimate: {
              testId: event.testId,
              expectedRevision: event.expectedRevision,
              candidateIds: [...event.candidateIds],
            },
            quotePlan: undefined,
            quote: undefined,
            failure: undefined,
            operation: context.operation + 1,
          }
        : {},
    ),
    quote: assign(({ event }) => ({ quote: quoteOf(event)!.quote })),
    freezeStart: assign(({ context }) => ({
      command: context.retryEstimate
        ? {
            kind: 'retry' as const,
            input: {
              ...structuredClone(context.retryEstimate),
              requestKey: context.requestKey(),
              quoteKey: context.quote!.quoteKey,
            },
          }
        : {
            kind: 'start' as const,
            input: {
              plan: structuredClone(context.quotePlan!),
              requestKey: context.requestKey(),
              quoteKey: context.quote!.quoteKey,
            },
          },
      uncertain: false,
      failure: undefined,
      operation: context.operation + 1,
    })),
    freezeVote: assign(({ context, event }) =>
      event.type === 'VOTE'
        ? {
            command: {
              kind: 'vote' as const,
              input: {
                testId: event.testId,
                expectedRevision: event.expectedRevision,
                matchId: event.matchId,
                winnerCandidateId: event.winnerCandidateId,
                requestKey: context.requestKey(),
              },
            },
            uncertain: false,
            failure: undefined,
            operation: context.operation + 1,
          }
        : {},
    ),
    freezeCancel: assign(({ context, event }) =>
      event.type === 'CANCEL'
        ? {
            command: {
              kind: 'cancel' as const,
              input: {
                testId: event.testId,
                expectedRevision: event.expectedRevision,
                requestKey: context.requestKey(),
              },
            },
            uncertain: false,
            failure: undefined,
            operation: context.operation + 1,
          }
        : {},
    ),
    freezePublish: assign(({ context, event }) =>
      event.type === 'PUBLISH' || event.type === 'APPLY'
        ? {
            command:
              event.type === 'PUBLISH'
                ? {
                    kind: 'publish' as const,
                    input: { ...structuredClone(event.input), requestKey: context.requestKey() },
                  }
                : {
                    kind: 'apply' as const,
                    input: { ...structuredClone(event.input), requestKey: context.requestKey() },
                  },
            publication: undefined,
            uncertain: false,
            failure: undefined,
            operation: context.operation + 1,
          }
        : {},
    ),
    retryOperation: assign(({ context }) => ({
      failure: undefined,
      operation: context.operation + 1,
    })),
    reload: assign(({ context }) => ({ failure: undefined, operation: context.operation + 1 })),
    response: assign(({ context, event }) => {
      const result = resultOf(event)!
      const publication = result.publication ?? matchingPublication(context.command, result.test)
      const pending = publication?.status === 'pending'
      return {
        test: result.test,
        testId: result.test.id,
        publication,
        command: pending ? context.command : undefined,
        uncertain: pending,
        quote: undefined,
        quotePlan: undefined,
        retryEstimate: undefined,
        failure:
          publication?.status === 'conflict'
            ? (publication.failure ?? {
                reason: 'WRITING_TEST_PUBLICATION_CONFLICT' as const,
                params: {},
              })
            : undefined,
      }
    }),
    hydrate: assign(({ context, event }) => {
      const result = resultOf(event)!
      const settles = settledCommand(context.command, result.test)
      const conflict = hydrationConflict(context.command, result.test)
      const publication = matchingPublication(context.command, result.test) ?? context.publication
      return {
        test: result.test,
        testId: result.test.id,
        publication,
        command: settles ? undefined : context.command,
        uncertain: !settles && !!context.command,
        failure: conflict
          ? {
              reason: publicationCommand(context.command)
                ? ('WRITING_TEST_PUBLICATION_CONFLICT' as const)
                : ('WRITING_TEST_DECISION_CONFLICT' as const),
              params: {},
            }
          : undefined,
        quote: undefined,
        quotePlan: undefined,
        retryEstimate: undefined,
      }
    }),
    workFailed: assign(({ event }) => {
      const failure = failureOf((event as unknown as { error: unknown }).error)
      return { failure, uncertain: isUncertain(failure) }
    }),
    readFailed: assign(({ event }) => ({
      failure: failureOf((event as unknown as { error: unknown }).error),
    })),
    invalidRead: assign({ failure: { reason: 'WRITING_TEST_REVISION_CONFLICT', params: {} } }),
    invalidResponse: assign({
      failure: { reason: 'UNKNOWN_FAILURE', params: {} },
      uncertain: true,
    }),
    dismissQuote: assign({ quote: undefined, quotePlan: undefined, retryEstimate: undefined }),
    clearQuoteCommand: assign({
      quote: undefined,
      quotePlan: undefined,
      command: undefined,
      uncertain: false,
    }),
    discardRefusedCommand: assign({ command: undefined, uncertain: false }),
    suspend: assign(({ context }) => ({ suspended: true, operation: context.operation + 1 })),
  },
}).createMachine({
  id: 'writingTestOperation',
  initial: 'initializing',
  context: ({ input }) => ({
    ownerId: input.ownerId,
    seedKey: input.seedKey,
    scopeKey: writingTestScopeKey(input.ownerId, input.seedKey),
    client: input.client,
    now: input.now ?? Date.now,
    requestKey: input.requestKey ?? (() => crypto.randomUUID()),
    draft: structuredClone(input.recovery?.draft ?? input.initialDraft),
    testId: input.testId ?? input.recovery?.testId,
    command: input.recovery?.command ? structuredClone(input.recovery.command) : undefined,
    uncertain: !!input.recovery?.command,
    operation: 0,
    suspended: false,
  }),
  on: {
    SUSPEND: { guard: 'owned', target: '.suspended', actions: 'suspend' },
  },
  states: {
    initializing: {
      always: [
        { guard: 'knownTest', target: 'loading' },
        { guard: 'uncertain', target: 'settling' },
        { target: 'editing' },
      ],
    },
    editing: {
      on: {
        DRAFT: { guard: 'canDraft', actions: 'draft' },
        ESTIMATE: [
          { guard: 'canEstimate', target: 'quoting', actions: 'freezeEstimate' },
          { guard: 'owned', actions: 'invalidDraft' },
        ],
      },
    },
    quoting: {
      invoke: {
        src: 'quote',
        input: ({ context }) => ({
          client: context.client,
          scopeKey: context.scopeKey,
          operation: context.operation,
          plan: context.quotePlan,
          retry: context.retryEstimate,
        }),
        onDone: { guard: 'validQuote', target: 'quoted', actions: 'quote' },
        onError: { target: 'failed', actions: 'readFailed' },
      },
      on: { QUOTED: { guard: 'validQuote', target: 'quoted', actions: 'quote' } },
    },
    quoted: {
      on: {
        CONFIRM: [
          { guard: 'canConfirm', target: 'admitting', actions: 'freezeStart' },
          { guard: 'expiredQuote', target: 'quoteExpired', actions: 'dismissQuote' },
        ],
        DISMISS_QUOTE: { guard: 'owned', target: 'settling', actions: 'dismissQuote' },
      },
    },
    admitting: {
      always: [
        { guard: ({ context }) => context.command?.kind === 'retry', target: 'retrying' },
        { target: 'starting' },
      ],
    },
    starting: { invoke: work },
    retrying: { invoke: work },
    deciding: { invoke: work },
    cancelling: { invoke: work },
    publishing: { invoke: work },
    loading: {
      invoke: read,
      on: { HYDRATE: { guard: 'validResult', target: 'settling', actions: 'hydrate' } },
    },
    settling: {
      always: [
        { guard: 'conflict', target: 'conflict' },
        { guard: 'uncertainPublication', target: 'uncertainPublication' },
        { guard: 'uncertain', target: 'uncertain' },
        { guard: 'expired', target: 'expired' },
        { guard: 'published', target: 'published' },
        { guard: 'cancelled', target: 'cancelled' },
        { guard: 'running', target: 'running' },
        { guard: 'partial', target: 'partial' },
        { guard: 'champion', target: 'champion' },
        { guard: 'match', target: 'match' },
        { guard: 'hasTest', target: 'failed' },
        { target: 'editing' },
      ],
    },
    workFailure: {
      always: [
        { guard: 'uncertainPublication', target: 'uncertainPublication' },
        { guard: 'uncertain', target: 'uncertain' },
        { guard: 'quoteRequired', target: 'quoteExpired', actions: 'clearQuoteCommand' },
        { guard: 'conflict', target: 'conflict', actions: 'discardRefusedCommand' },
        { target: 'failed', actions: 'discardRefusedCommand' },
      ],
    },
    readFailure: {
      always: [
        { guard: 'uncertainPublication', target: 'uncertainPublication' },
        { guard: 'uncertain', target: 'uncertain' },
        { target: 'failed' },
      ],
    },
    running: {
      on: {
        RELOAD: reload,
        CANCEL: { guard: 'canCancel', target: 'cancelling', actions: 'freezeCancel' },
      },
    },
    partial: {
      on: {
        RELOAD: reload,
        ESTIMATE_RETRY: {
          guard: 'canRetryEstimate',
          target: 'quoting',
          actions: 'freezeRetryEstimate',
        },
        CANCEL: { guard: 'canCancel', target: 'cancelling', actions: 'freezeCancel' },
      },
    },
    match: {
      on: {
        RELOAD: reload,
        VOTE: { guard: 'canVote', target: 'deciding', actions: 'freezeVote' },
        CANCEL: { guard: 'canCancel', target: 'cancelling', actions: 'freezeCancel' },
      },
    },
    champion: {
      on: {
        RELOAD: reload,
        PUBLISH: { guard: 'canPublish', target: 'publishing', actions: 'freezePublish' },
        APPLY: { guard: 'canApply', target: 'publishing', actions: 'freezePublish' },
      },
    },
    published: {
      on: {
        RELOAD: reload,
        PUBLISH: { guard: 'canPublish', target: 'publishing', actions: 'freezePublish' },
        APPLY: { guard: 'canApply', target: 'publishing', actions: 'freezePublish' },
      },
    },
    cancelled: { on: { RELOAD: reload } },
    expired: { on: { RELOAD: reload } },
    conflict: {
      on: {
        RELOAD: reload,
        PUBLISH: { guard: 'canPublish', target: 'publishing', actions: 'freezePublish' },
      },
    },
    quoteExpired: {
      on: {
        DRAFT: { guard: 'canDraft', actions: 'draft' },
        ESTIMATE: { guard: 'canEstimate', target: 'quoting', actions: 'freezeEstimate' },
        ESTIMATE_RETRY: {
          guard: 'canRetryEstimate',
          target: 'quoting',
          actions: 'freezeRetryEstimate',
        },
        RELOAD: reload,
      },
    },
    uncertain: {
      on: {
        RELOAD: reload,
        RETRY_OPERATION: [
          { guard: 'retryStart', target: 'starting', actions: 'retryOperation' },
          { guard: 'retryVote', target: 'deciding', actions: 'retryOperation' },
          { guard: 'retryCandidates', target: 'retrying', actions: 'retryOperation' },
          { guard: 'retryCancel', target: 'cancelling', actions: 'retryOperation' },
        ],
      },
    },
    uncertainPublication: {
      on: {
        RELOAD: reload,
        RETRY_OPERATION: { guard: 'retryPublish', target: 'publishing', actions: 'retryOperation' },
      },
    },
    failed: {
      on: {
        RELOAD: reload,
        DRAFT: { guard: 'canDraft', target: 'editing', actions: 'draft' },
        ESTIMATE: { guard: 'canEstimate', target: 'quoting', actions: 'freezeEstimate' },
        ESTIMATE_RETRY: {
          guard: 'canRetryEstimate',
          target: 'quoting',
          actions: 'freezeRetryEstimate',
        },
        CANCEL: { guard: 'canCancel', target: 'cancelling', actions: 'freezeCancel' },
      },
    },
    suspended: {},
  },
})
export type WritingTestOperationSnapshot = SnapshotFrom<typeof writingTestMachine>
export function writingTestPhase(snapshot: WritingTestOperationSnapshot): WritingTestPhase {
  return snapshot.value as WritingTestPhase
}
export function writingTestOperationBusy(phase: WritingTestPhase): boolean {
  return [
    'loading',
    'quoting',
    'starting',
    'retrying',
    'deciding',
    'publishing',
    'cancelling',
  ].includes(phase)
}
