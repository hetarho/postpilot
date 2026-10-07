import type { AppFailure, ContentLanguage, PostContent } from '@/shared/api'

export const TEST_COUNTS = [2, 4, 8, 16] as const
export type TestCount = (typeof TEST_COUNTS)[number]
export type TestFactor = 'model' | 'voice' | 'template' | 'guideline'
export type TestStage = 'observe' | 'write'
export type TestStatus =
  'queued' | 'running' | 'partial' | 'review' | 'completed' | 'cancelled' | 'failed'
export type TestCandidateStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type TestSettingKind = 'writing-voice' | 'post-template' | 'post-guideline'
export type TestQualityRule =
  'title-saturation' | 'cross-post-phrases' | 'in-post-repetition' | 'composition'
export interface TestModelRef {
  providerId: string
  modelId: string
}
export type TestEntrant =
  | { type: 'model'; model: TestModelRef }
  | { type: 'setting'; setting: { kind: TestSettingKind; id: string; revision: string } }
  | { type: 'authoring'; authoring: { sessionId: string; candidateId: string; revision: number } }
export interface TestMaterial {
  text: string
  fictional: boolean
  attachmentIds: string[]
  templateAnswers: { label: string; text: string; enabled: boolean }[]
}
export interface WritingTestContext {
  sourcePostSlug: string
  expectedInputRevision: bigint
  expectedContentRevision: bigint
  material: TestMaterial
  observeModel?: TestModelRef
  writeModel?: TestModelRef
  voiceId: string
  templateId: string
  guidelineSlotId: string
  targetLanguage: ContentLanguage
  targetLength: number
  tagCount: number
  useMemory: boolean
  qualityRules: TestQualityRule[]
}
export interface WritingTestPlan {
  factor: TestFactor
  modelStage: TestStage
  count: TestCount
  entrants: TestEntrant[]
  context: WritingTestContext
}
export interface WritingTestQuote {
  free: boolean
  credits: number
  quoteKey: string
  expiresAt: string
}
export interface WritingTestIdentity {
  label: string
  source: TestEntrant
  synthetic: boolean
}
export interface WritingTestUsage {
  promptTokens: number
  completionTokens: number
  latencyMs: number
}
export interface TestStoryline {
  paragraphs: { text: string; files: string[] }[]
  editedByHand: boolean
  addedFiles: string[]
  takenOutFiles: string[]
}
export interface WritingTestCandidate {
  id: string
  status: TestCandidateStatus
  output?: PostContent
  failure?: AppFailure
  identity?: WritingTestIdentity
  displayLabel: string
  storyline?: TestStoryline
  usage?: WritingTestUsage
}
export interface WritingTestMatch {
  id: string
  round: number
  index: number
  leftCandidateId: string
  rightCandidateId: string
  winnerCandidateId: string
}
export type TestPublicationAction = 'save-setting' | 'use-setting' | 'adopt-model' | 'apply-output'
export interface WritingTestPublication {
  id: string
  testId: string
  winnerCandidateId: string
  action: TestPublicationAction
  status: 'pending' | 'confirmed' | 'conflict'
  requestKey: string
  targetId: string
  failure?: AppFailure
}
export interface WritingTest {
  id: string
  /** uint32 server revision; post input/content revisions use bigint separately. */
  revision: number
  factor: TestFactor
  modelStage: TestStage
  count: TestCount
  status: TestStatus
  sourcePostSlug: string
  jobId: string
  candidates: WritingTestCandidate[]
  matches: WritingTestMatch[]
  winnerCandidateId: string
  publications: WritingTestPublication[]
  revealed: boolean
  createdAt: string
  updatedAt: string
  contentExpiresAt: string
  fictional: boolean
  confirmedCredits: number
  reservedCredits: number
  failure?: AppFailure
  /** Frozen output provenance, never a current locale or source-post target. */
  targetLanguage: ContentLanguage
}
export interface TestRevisionInput {
  testId: string
  expectedRevision: number
}
export interface TestOperationInput extends TestRevisionInput {
  requestKey: string
}
export interface TestRetryEstimateInput extends TestRevisionInput {
  candidateIds: string[]
}
export interface TestRetryInput extends TestRetryEstimateInput, TestOperationInput {
  quoteKey: string
}
export interface TestStartInput {
  plan: WritingTestPlan
  requestKey: string
  quoteKey: string
}
export interface TestDecisionInput extends TestOperationInput {
  matchId: string
  winnerCandidateId: string
}
export interface TestWinnerInput extends TestOperationInput {
  winnerCandidateId: string
  action: Exclude<TestPublicationAction, 'apply-output'>
  makeDefault: boolean
  name: string
  scope: string
  scopeIds: string[]
}
export interface TestApplyInput extends TestOperationInput {
  winnerCandidateId: string
  expectedInputRevision: bigint
  expectedContentRevision: bigint
}
export interface TestListInput {
  pageSize?: number
  pageToken?: string
}
export interface TestList {
  tests: WritingTest[]
  nextPageToken: string
}
export interface TestPublicationResult {
  test: WritingTest
  publication: WritingTestPublication
}
export interface WritingTestClient {
  estimate(plan: WritingTestPlan, signal?: AbortSignal): Promise<WritingTestQuote>
  estimateRetry(input: TestRetryEstimateInput, signal?: AbortSignal): Promise<WritingTestQuote>
  start(input: TestStartInput, signal?: AbortSignal): Promise<WritingTest>
  get(testId: string, signal?: AbortSignal): Promise<WritingTest>
  list(input?: TestListInput, signal?: AbortSignal): Promise<TestList>
  retry(input: TestRetryInput, signal?: AbortSignal): Promise<WritingTest>
  decide(input: TestDecisionInput, signal?: AbortSignal): Promise<WritingTest>
  cancel(input: TestOperationInput, signal?: AbortSignal): Promise<WritingTest>
  saveWinner(input: TestWinnerInput, signal?: AbortSignal): Promise<TestPublicationResult>
  applyOutput(input: TestApplyInput, signal?: AbortSignal): Promise<TestPublicationResult>
}
export function writingTestBusy(test?: WritingTest): boolean {
  return !!test && (test.status === 'queued' || test.status === 'running')
}
export function writingTestPayloadAvailable(test: WritingTest, now = Date.now()): boolean {
  return (
    (!test.contentExpiresAt || Date.parse(test.contentExpiresAt) > now) &&
    test.candidates.some((candidate) => !!candidate.output)
  )
}
