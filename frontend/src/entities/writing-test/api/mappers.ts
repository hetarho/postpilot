import { clone, create } from '@bufbuild/protobuf'
import {
  appFailureFromProto,
  BlockType,
  contentLanguageToProto,
  PostContentSchema,
  ProtoConfigurationKind,
  ProtoQualityMetric,
  requireContentLanguage,
  WritingTestCandidateStatus as CandidateStatus,
  WritingTestFactor as Factor,
  WritingTestStage as Stage,
  WritingTestStatus as Status,
  WritingTestPublicationAction as Action,
  WritingTestPublicationStatus as PublicationStatus,
  WritingTestPlanSchema,
  type EstimateWritingTestResponse,
  type ProtoFailure,
  type WritingTest as WireTest,
  type WritingTestEntrant as WireEntrant,
  type WritingTestPublication as WirePublication,
} from '@/shared/api'
import {
  TEST_COUNTS,
  type TestCandidateStatus,
  type TestCount,
  type TestEntrant,
  type TestFactor,
  type TestModelRef,
  type TestPublicationAction,
  type TestQualityRule,
  type TestSettingKind,
  type TestStage,
  type TestStatus,
  type WritingTest,
  type WritingTestPlan,
  type WritingTestPublication,
  type WritingTestQuote,
} from '../model/types'

export class WritingTestValidationError extends Error {
  constructor(public readonly field: string) {
    super(`Writing test ${field} is invalid`)
    this.name = 'WritingTestValidationError'
  }
}
const invalid = (field: string): never => {
  throw new WritingTestValidationError(field)
}
export function requireText(value: string, field: string): string {
  if (typeof value !== 'string' || !value.trim()) invalid(field)
  return value
}
export function requireRevision(value: number): number {
  if (!Number.isInteger(value) || value < 0 || value > 0xffffffff) invalid('revision')
  return value
}
export function requirePostRevision(value: bigint): bigint {
  if (typeof value !== 'bigint' || value < 0n || value > 9223372036854775807n)
    invalid('post revision')
  return value
}
export function safeCount(value: bigint, field: string): number {
  if (typeof value !== 'bigint' || value < 0n || value > BigInt(Number.MAX_SAFE_INTEGER))
    invalid(field)
  return Number(value)
}
const factorMap: Record<TestFactor, Factor> = {
  model: Factor.MODEL,
  voice: Factor.VOICE,
  template: Factor.TEMPLATE,
  guideline: Factor.GUIDELINE,
}
const stageMap: Record<TestStage, Stage> = { observe: Stage.OBSERVE, write: Stage.WRITE }
const statusMap: Record<TestStatus, Status> = {
  queued: Status.QUEUED,
  running: Status.RUNNING,
  partial: Status.PARTIAL,
  review: Status.REVIEW,
  completed: Status.COMPLETED,
  cancelled: Status.CANCELLED,
  failed: Status.FAILED,
}
const candidateMap: Record<TestCandidateStatus, CandidateStatus> = {
  pending: CandidateStatus.PENDING,
  running: CandidateStatus.RUNNING,
  succeeded: CandidateStatus.SUCCEEDED,
  failed: CandidateStatus.FAILED,
  cancelled: CandidateStatus.CANCELLED,
}
const settingMap: Record<TestSettingKind, ProtoConfigurationKind> = {
  'writing-voice': ProtoConfigurationKind.WRITING_VOICE,
  'post-template': ProtoConfigurationKind.POST_TEMPLATE,
  'post-guideline': ProtoConfigurationKind.POST_GUIDELINE,
}
const qualityMap: Record<TestQualityRule, ProtoQualityMetric> = {
  'title-saturation': ProtoQualityMetric.TITLE_SATURATION,
  'cross-post-phrases': ProtoQualityMetric.CROSS_POST_PHRASES,
  'in-post-repetition': ProtoQualityMetric.IN_POST_REPETITION,
  composition: ProtoQualityMetric.COMPOSITION,
}
const actionMap: Record<TestPublicationAction, Action> = {
  'save-setting': Action.SAVE_SETTING,
  'use-setting': Action.USE_SETTING,
  'adopt-model': Action.ADOPT_MODEL,
  'apply-output': Action.APPLY_OUTPUT,
}
const publicationMap: Record<WritingTestPublication['status'], PublicationStatus> = {
  pending: PublicationStatus.PENDING,
  confirmed: PublicationStatus.CONFIRMED,
  conflict: PublicationStatus.CONFLICT,
}
function fromEnum<K extends string>(map: Record<K, number>, value: number, field: string): K {
  const name = (Object.keys(map) as K[]).find((key) => map[key] === value)
  return name ?? invalid(field)
}
function toEnum<K extends string>(map: Record<K, number>, value: K, field: string): number {
  return map[value] ?? invalid(field)
}
function count(value: number): TestCount {
  return TEST_COUNTS.includes(value as TestCount) ? (value as TestCount) : invalid('count')
}
function date(value: string, field: string, required = false): string {
  if ((required && !value) || (value && !Number.isFinite(Date.parse(value)))) invalid(field)
  return value
}
function model(value: TestModelRef): TestModelRef {
  return {
    providerId: requireText(value.providerId, 'model provider'),
    modelId: requireText(value.modelId, 'model'),
  }
}
function failure(value: ProtoFailure | undefined, revealed = true) {
  if (!value) return undefined
  // Technical details and unknown params never belong in public results, especially blind ones.
  return appFailureFromProto({
    ...value,
    params: revealed ? value.params : {},
    technicalDetail: '',
  })
}
export function entrantFromProto(value: WireEntrant): TestEntrant {
  switch (value.source.case) {
    case 'model':
      return { type: 'model', model: model(value.source.value) }
    case 'setting':
      return {
        type: 'setting',
        setting: {
          kind: fromEnum(settingMap, value.source.value.kind, 'setting kind'),
          id: requireText(value.source.value.id, 'setting'),
          revision: requireText(value.source.value.revision, 'setting revision'),
        },
      }
    case 'authoringCandidate':
      return {
        type: 'authoring',
        authoring: {
          sessionId: requireText(value.source.value.sessionId, 'authoring session'),
          candidateId: requireText(value.source.value.candidateId, 'authoring candidate'),
          revision: requireRevision(value.source.value.revision),
        },
      }
    default:
      return invalid('entrant')
  }
}
function entrantToProto(value: TestEntrant) {
  switch (value.type) {
    case 'model':
      return { source: { case: 'model' as const, value: model(value.model) } }
    case 'setting':
      return {
        source: {
          case: 'setting' as const,
          value: {
            kind: toEnum(settingMap, value.setting.kind, 'setting kind'),
            id: requireText(value.setting.id, 'setting'),
            revision: requireText(value.setting.revision, 'setting revision'),
          },
        },
      }
    case 'authoring':
      return {
        source: {
          case: 'authoringCandidate' as const,
          value: {
            sessionId: requireText(value.authoring.sessionId, 'authoring session'),
            candidateId: requireText(value.authoring.candidateId, 'authoring candidate'),
            revision: requireRevision(value.authoring.revision),
          },
        },
      }
    default:
      return invalid('entrant')
  }
}
export function entrantKey(value: TestEntrant): string {
  switch (value.type) {
    case 'model':
      return JSON.stringify(['model', value.model.providerId, value.model.modelId])
    case 'setting':
      return JSON.stringify(['setting', value.setting.kind, value.setting.id])
    case 'authoring':
      return JSON.stringify(['authoring', value.authoring.sessionId, value.authoring.candidateId])
    default:
      return invalid('entrant')
  }
}
export function planToProto(plan: WritingTestPlan) {
  count(plan.count)
  const factor = toEnum(factorMap, plan.factor, 'factor')
  const modelStage = toEnum(stageMap, plan.modelStage, 'stage')
  if (plan.factor !== 'model' && plan.modelStage !== 'write') invalid('stage')
  if (plan.entrants.length !== plan.count) invalid('entrant count')
  const expectedKind: Partial<Record<TestFactor, TestSettingKind>> = {
    voice: 'writing-voice',
    template: 'post-template',
    guideline: 'post-guideline',
  }
  if (
    plan.entrants.some((entrant) =>
      plan.factor === 'model'
        ? entrant.type !== 'model'
        : entrant.type === 'model' ||
          (entrant.type === 'setting' && entrant.setting.kind !== expectedKind[plan.factor]),
    )
  )
    invalid('entrant factor')
  if (new Set(plan.entrants.map(entrantKey)).size !== plan.count) invalid('duplicate entrants')
  const context = plan.context
  if (!['ko', 'en'].includes(context.targetLanguage)) invalid('target language')
  requirePostRevision(context.expectedInputRevision)
  requirePostRevision(context.expectedContentRevision)
  if (
    !Number.isInteger(context.targetLength) ||
    context.targetLength < 0 ||
    context.targetLength > 0x7fffffff
  )
    invalid('target length')
  if (!Number.isInteger(context.tagCount) || context.tagCount < 0 || context.tagCount > 0x7fffffff)
    invalid('tag count')
  if (!context.material.text.trim() && context.material.attachmentIds.length === 0)
    invalid('material')
  if (
    new Set(context.material.attachmentIds).size !== context.material.attachmentIds.length ||
    context.material.attachmentIds.some((id) => !id.trim())
  )
    invalid('attachments')
  if (
    plan.factor === 'model' &&
    plan.modelStage === 'observe' &&
    context.material.attachmentIds.length === 0
  )
    invalid('observer attachments')
  if ((plan.factor !== 'model' || plan.modelStage === 'observe') && !context.writeModel)
    invalid('fixed writer')
  if (plan.factor === 'guideline' && !context.guidelineSlotId.trim()) invalid('guideline slot')
  if (new Set(context.qualityRules).size !== context.qualityRules.length) invalid('quality rules')
  if (
    new Set(context.material.templateAnswers.map((answer) => answer.label)).size !==
      context.material.templateAnswers.length ||
    context.material.templateAnswers.some((answer) => !answer.label.trim())
  )
    invalid('template answers')
  return create(WritingTestPlanSchema, {
    factor,
    modelStage,
    count: plan.count,
    entrants: plan.entrants.map(entrantToProto),
    context: {
      ...context,
      targetLanguage: contentLanguageToProto(context.targetLanguage),
      observeModel: context.observeModel ? model(context.observeModel) : undefined,
      writeModel: context.writeModel ? model(context.writeModel) : undefined,
      material: {
        ...context.material,
        attachmentIds: [...context.material.attachmentIds],
        templateAnswers: context.material.templateAnswers.map((answer) => ({ ...answer })),
      },
      qualityRules: context.qualityRules.map((rule) => toEnum(qualityMap, rule, 'quality rule')),
    },
  })
}
export function mapWritingTestQuote(wire: EstimateWritingTestResponse): WritingTestQuote {
  const credits =
    wire.free && wire.credits === undefined
      ? 0
      : safeCount(wire.credits ?? invalid('quote credits'), 'quote credits')
  if (wire.free && credits !== 0) invalid('free quote credits')
  return {
    free: wire.free,
    credits,
    quoteKey: requireText(wire.quoteKey, 'quote'),
    expiresAt: date(wire.expiresAt, 'quote expiry', true),
  }
}
export const publicationActionToProto = (value: TestPublicationAction) =>
  toEnum(actionMap, value, 'publication action')
export function mapWritingTestPublication(
  wire: WirePublication,
  expectedTestId?: string,
): WritingTestPublication {
  requireText(wire.id, 'publication')
  requireText(wire.testId, 'publication test')
  if (expectedTestId && wire.testId !== expectedTestId) invalid('publication test mismatch')
  const status = fromEnum(publicationMap, wire.status, 'publication status')
  if (status === 'confirmed' && !wire.targetId) invalid('publication target')
  return {
    id: wire.id,
    testId: wire.testId,
    winnerCandidateId: requireText(wire.winnerCandidateId, 'publication winner'),
    action: fromEnum(actionMap, wire.action, 'publication action'),
    status,
    requestKey: requireText(wire.requestKey, 'publication request'),
    targetId: wire.targetId,
    failure: failure(wire.failure),
  }
}
export function mapWritingTest(wire: WireTest, expectedId?: string): WritingTest {
  requireText(wire.id, 'identity')
  if (expectedId && wire.id !== expectedId) invalid('identity mismatch')
  const entrantCount = count(wire.count)
  const factor = fromEnum(factorMap, wire.factor, 'factor')
  const modelStage = fromEnum(stageMap, wire.modelStage, 'stage')
  const status = fromEnum(statusMap, wire.status, 'status')
  if (wire.revealed && !['completed', 'cancelled'].includes(status)) invalid('premature reveal')
  if (factor !== 'model' && modelStage !== 'write') invalid('stage')
  if (
    wire.candidates.length !== entrantCount ||
    new Set(wire.candidates.map((candidate) => candidate.id)).size !== entrantCount
  )
    invalid('candidates')
  const candidates = wire.candidates.map((candidate) => {
    requireText(candidate.id, 'candidate')
    const candidateStatus = fromEnum(candidateMap, candidate.status, 'candidate status')
    if (
      candidate.output &&
      (!candidate.output.title.trim() ||
        candidate.output.blocks.length === 0 ||
        candidate.output.blocks.some(
          (block) =>
            ![
              BlockType.TEXT,
              BlockType.HEADING,
              BlockType.IMAGE,
              BlockType.QUOTE,
              BlockType.LIST,
              BlockType.VIDEO,
              BlockType.GALLERY,
            ].includes(block.type),
        ))
    )
      invalid('post output')
    return {
      id: candidate.id,
      status: candidateStatus,
      output: candidate.output ? clone(PostContentSchema, candidate.output) : undefined,
      failure: failure(candidate.failure, wire.revealed),
      identity:
        wire.revealed && candidate.identity
          ? {
              label: requireText(candidate.identity.label, 'identity label'),
              source: candidate.identity.source
                ? entrantFromProto(candidate.identity.source)
                : invalid('identity source'),
              synthetic: candidate.identity.synthetic,
            }
          : undefined,
      // A bad backend label must not reveal a model/setting before the privacy boundary.
      displayLabel: wire.revealed
        ? candidate.displayLabel
        : `#${wire.candidates.indexOf(candidate) + 1}`,
      storyline: candidate.storyline
        ? {
            paragraphs: candidate.storyline.paragraphs.map((paragraph) => ({
              text: paragraph.text,
              files: [...paragraph.files],
            })),
            editedByHand: candidate.storyline.editedByHand,
            addedFiles: [...candidate.storyline.addedFiles],
            takenOutFiles: [...candidate.storyline.takenOutFiles],
          }
        : undefined,
      usage:
        wire.revealed && candidate.usage
          ? {
              promptTokens: safeCount(candidate.usage.promptTokens, 'prompt tokens'),
              completionTokens: safeCount(candidate.usage.completionTokens, 'completion tokens'),
              latencyMs: safeCount(candidate.usage.latencyMs, 'latency'),
            }
          : undefined,
    }
  })
  const ids = new Set(candidates.map((candidate) => candidate.id))
  if (
    ['review', 'completed'].includes(status) &&
    candidates.some((candidate) => candidate.status !== 'succeeded')
  )
    invalid('complete output barrier')
  if (
    wire.matches.length > entrantCount - 1 ||
    new Set(wire.matches.map((match) => match.id)).size !== wire.matches.length
  )
    invalid('matches')
  const positions = new Set<string>()
  const matches = wire.matches.map((match) => {
    requireText(match.id, 'match')
    if (
      !Number.isInteger(match.round) ||
      match.round < 1 ||
      match.round > Math.log2(entrantCount) ||
      !Number.isInteger(match.index) ||
      match.index < 0 ||
      match.index >= entrantCount / 2 ** match.round
    )
      invalid('match position')
    const position = `${match.round}:${match.index}`
    if (positions.has(position)) invalid('duplicate match position')
    positions.add(position)
    if (
      (match.leftCandidateId && !ids.has(match.leftCandidateId)) ||
      (match.rightCandidateId && !ids.has(match.rightCandidateId)) ||
      (match.leftCandidateId && match.leftCandidateId === match.rightCandidateId) ||
      (match.winnerCandidateId &&
        (!match.leftCandidateId ||
          !match.rightCandidateId ||
          ![match.leftCandidateId, match.rightCandidateId].includes(match.winnerCandidateId)))
    )
      invalid('match contestants')
    return {
      id: match.id,
      round: match.round,
      index: match.index,
      leftCandidateId: match.leftCandidateId,
      rightCandidateId: match.rightCandidateId,
      winnerCandidateId: match.winnerCandidateId,
    }
  })
  for (const match of matches) {
    if (match.round === 1) continue
    const left = matches.find(
      (child) => child.round === match.round - 1 && child.index === match.index * 2,
    )
    const right = matches.find(
      (child) => child.round === match.round - 1 && child.index === match.index * 2 + 1,
    )
    if (
      (match.leftCandidateId && left?.winnerCandidateId !== match.leftCandidateId) ||
      (match.rightCandidateId && right?.winnerCandidateId !== match.rightCandidateId)
    )
      invalid('bracket advancement')
  }
  if (['review', 'completed'].includes(status)) {
    const firstRound = matches.filter((match) => match.round === 1)
    const firstIds = firstRound.flatMap((match) => [match.leftCandidateId, match.rightCandidateId])
    if (
      firstRound.length !== entrantCount / 2 ||
      firstIds.length !== new Set(firstIds).size ||
      firstIds.some((id) => !ids.has(id))
    )
      invalid('initial bracket')
  }
  if (wire.winnerCandidateId && !ids.has(wire.winnerCandidateId)) invalid('winner')
  if (
    status === 'completed' &&
    (!wire.revealed ||
      !wire.winnerCandidateId ||
      matches.filter((match) => match.winnerCandidateId).length !== entrantCount - 1 ||
      matches.find((match) => match.round === Math.log2(entrantCount))?.winnerCandidateId !==
        wire.winnerCandidateId)
  )
    invalid('champion confirmation')
  const publications = wire.publications.map((publication) =>
    mapWritingTestPublication(publication, wire.id),
  )
  if (
    new Set(publications.map((publication) => publication.id)).size !== publications.length ||
    publications.some((publication) => publication.winnerCandidateId !== wire.winnerCandidateId)
  )
    invalid('publications')
  return {
    id: wire.id,
    revision: requireRevision(wire.revision),
    factor,
    modelStage,
    count: entrantCount,
    status,
    sourcePostSlug: wire.sourcePostSlug,
    jobId: wire.jobId,
    candidates,
    matches,
    winnerCandidateId: wire.winnerCandidateId,
    publications,
    revealed: wire.revealed,
    createdAt: date(wire.createdAt, 'created date'),
    updatedAt: date(wire.updatedAt, 'updated date'),
    contentExpiresAt: date(wire.contentExpiresAt, 'payload expiry'),
    fictional: wire.fictional,
    confirmedCredits: safeCount(wire.confirmedCredits, 'confirmed credits'),
    reservedCredits: safeCount(wire.reservedCredits, 'reserved credits'),
    failure: failure(wire.failure, wire.revealed),
    targetLanguage: requireContentLanguage(wire.targetLanguage),
  }
}
