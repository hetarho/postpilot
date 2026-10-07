import { create, type MessageInitShape, type MessageShape } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import {
  AuthService,
  ConfigurationAuthoringService as Authoring,
  GuidelineService,
  ModelExperimentService,
  PostService,
  ProtoConfigurationKind,
  ProtoPlan,
  ProviderService,
  Stage,
  TemplateService,
  TemplateSchema,
  VoiceService,
  VoiceSchema,
  GuidelineSchema,
  WritingTestIdentitySchema,
  WritingTestPublicationAction,
  WritingTestPublicationStatus,
  WritingTestPublicationSchema,
  WritingTestService as Service,
  WritingTestStatus,
  contentLanguageToProto,
  type StartWritingTestRequest,
  type WritingTestPlan as WirePlan,
  type DecideTestMatchRequest,
  type SaveWritingTestWinnerRequest,
  type ApplyWritingTestOutputRequest,
  type WritingTest as WireTest,
} from '@/shared/api'
import type { TestCount, TestFactor, WritingTestPlan } from '../model/types'
import { wireTest } from './fixtures.test-support'

/** Deterministic RPC fixture used by real actors in host tests and temporary browser previews. */
export function createWritingTestStudioFixture(
  options: {
    count?: TestCount
    factor?: TestFactor
    source?: boolean
    readableTest?: boolean
    generationGate?: Promise<void>
    estimateFailure?: boolean
    ownedVoices?: MessageInitShape<typeof VoiceSchema>[]
    ownedTemplates?: MessageInitShape<typeof TemplateSchema>[]
    ownedGuidelines?: MessageInitShape<typeof GuidelineSchema>[]
    /** Each explicit batch may produce a different required-input contract. */
    preparationBodies?: string[]
    comparisonPairs?: Array<{
      stage: Stage
      candidateA: { providerId: string; modelId: string }
      candidateB: { providerId: string; modelId: string }
      extraCandidates?: Array<{ providerId: string; modelId: string }>
    }>
  } = {},
) {
  const count = options.count ?? 2
  const factor = options.factor ?? 'model'
  const calls: string[] = []
  const estimates: WirePlan[] = []
  const admissions: StartWritingTestRequest[] = []
  const votes: DecideTestMatchRequest[] = []
  const publications: SaveWritingTestWinnerRequest[] = []
  const applications: ApplyWritingTestOutputRequest[] = []
  const preparations: {
    kind: ProtoConfigurationKind
    count: number
    prompt: string
    requestKey: string
  }[] = []
  const preparationEstimates: { kind: ProtoConfigurationKind; count: number }[] = []
  type Session = NonNullable<
    MessageShape<typeof Authoring.method.getAuthoringSession.output>['session']
  >
  const preparationSessions = new Map<string, { ownerId: string; session: Session }>()
  const preparationCreates: string[] = []
  const createRequests = new Map<string, string>()
  let ownerId = 'alice'
  let estimatesToRefuse = options.estimateFailure ? 1 : 0
  let ledgerOwnerId = 'alice'
  let admittedPlan: WirePlan | undefined
  let ledger: WireTest | undefined = options.readableTest ? wireTest(count) : undefined
  const plan: WritingTestPlan = {
    factor,
    modelStage: 'write',
    count,
    entrants: [],
    context: {
      sourcePostSlug: options.source ? 'owned-source' : '',
      expectedInputRevision: options.source ? 9007199254740993n : 0n,
      expectedContentRevision: options.source ? 7n : 0n,
      material: {
        text: options.source ? 'Real recorded material' : 'Same fictional writing material',
        fictional: !options.source,
        attachmentIds: options.source ? ['owned-photo'] : [],
        templateAnswers: [],
      },
      writeModel: factor === 'model' ? undefined : { providerId: 'openrouter', modelId: 'model-0' },
      observeModel: options.source ? { providerId: 'openrouter', modelId: 'model-0' } : undefined,
      voiceId: '',
      templateId: '',
      guidelineSlotId: factor === 'guideline' ? 'designated-slot' : '',
      targetLanguage: options.source ? 'en' : 'ko',
      targetLength: 1000,
      tagCount: 4,
      useMemory: false,
      qualityRules: [],
    },
  }
  const transport = createRouterTransport(({ rpc }) => {
    rpc(AuthService.method.getMe, () => {
      calls.push('GetMe')
      return create(AuthService.method.getMe.output, {
        user: { id: ownerId, emailVerified: true, hasPassword: true },
        plan: ProtoPlan.MASTER,
      })
    })
    rpc(ProviderService.method.listModels, () => {
      calls.push('ListModels')
      return create(ProviderService.method.listModels.output, {
        models: Array.from({ length: 16 }, (_, index) => ({
          ref: { providerId: 'openrouter', modelId: `model-${index}` },
          label: `Model ${index + 1}`,
          vision: true,
          structuredOutput: true,
          affordable: true,
          stages: [Stage.WRITE, Stage.OBSERVE],
          levels: [Stage.WRITE, Stage.OBSERVE].map((stage) => ({ stage, level: 'balanced' })),
          access: [Stage.WRITE, Stage.OBSERVE].map((stage) => ({
            stage,
            grade: 'balanced',
            requiredPlan: 'pro',
            entitled: true,
            freePathAvailable: false,
          })),
        })),
      })
    })
    rpc(ProviderService.method.getSelections, () => {
      calls.push('GetSelections')
      return create(ProviderService.method.getSelections.output, {
        selections: [Stage.WRITE, Stage.OBSERVE].map((stage) => ({
          stage,
          ref: { providerId: 'openrouter', modelId: 'model-0' },
        })),
      })
    })
    rpc(ProviderService.method.getComparisonPairs, () => {
      calls.push('GetComparisonPairs')
      return create(ProviderService.method.getComparisonPairs.output, {
        pairs: (options.comparisonPairs ?? []).map((pair) => ({
          stage: pair.stage,
          candidateA: { stage: pair.stage, ref: pair.candidateA },
          candidateB: { stage: pair.stage, ref: pair.candidateB },
          extraCandidates: pair.extraCandidates?.map((ref) => ({ stage: pair.stage, ref })),
        })),
      })
    })
    rpc(VoiceService.method.listVoices, () => {
      calls.push('ListVoices')
      return create(VoiceService.method.listVoices.output, { voices: options.ownedVoices ?? [] })
    })
    rpc(VoiceService.method.listVoiceChecks, () => {
      calls.push('ListVoiceChecks')
      return create(VoiceService.method.listVoiceChecks.output, {})
    })
    rpc(TemplateService.method.listTemplates, () => {
      calls.push('ListTemplates')
      return create(TemplateService.method.listTemplates.output, {
        templates: options.ownedTemplates ?? [],
      })
    })
    rpc(GuidelineService.method.listGuidelines, () => {
      calls.push('ListGuidelines')
      return create(GuidelineService.method.listGuidelines.output, {
        guidelines: options.ownedGuidelines ?? [],
      })
    })
    rpc(ModelExperimentService.method.listExperiments, () => {
      calls.push('ListExperiments')
      return create(ModelExperimentService.method.listExperiments.output, {})
    })
    rpc(PostService.method.listPosts, () => {
      calls.push('ListPosts')
      return create(PostService.method.listPosts.output, {
        posts: options.source
          ? [
              {
                slug: 'owned-source',
                title: 'Owned material',
                status: 'draft',
                updatedAt: '2026-10-07T00:00:00Z',
              },
            ]
          : [],
      })
    })
    rpc(PostService.method.getPost, (request) => {
      calls.push('GetPost')
      if (request.slug !== 'owned-source' || !options.source)
        throw new ConnectError('not found', Code.NotFound)
      return create(PostService.method.getPost.output, {
        post: {
          slug: 'owned-source',
          title: 'Owned material',
          memo: 'Real recorded material',
          status: 'draft',
          inputRevision: 9007199254740993n,
          contentRevision: 7n,
          targetLanguage: contentLanguageToProto('en'),
          targetLength: 1000,
          tagCount: 4,
          images: [
            { id: 'owned-photo', filename: 'owned.jpg', width: 800, height: 600, bytes: 2000n },
          ],
        },
      })
    })
    rpc(Authoring.method.estimateAuthoringOperation, (request) => {
      calls.push('EstimateAuthoringOperation')
      preparationEstimates.push({ kind: request.kind, count: request.candidateCount })
      return create(Authoring.method.estimateAuthoringOperation.output, {
        credits: BigInt(request.candidateCount),
      })
    })
    rpc(Authoring.method.createAuthoringSession, (request) => {
      calls.push('CreateAuthoringSession')
      const requestKey = JSON.stringify([ownerId, request.requestId])
      const existingId = createRequests.get(requestKey)
      if (existingId)
        return create(Authoring.method.createAuthoringSession.output, {
          session: preparationSessions.get(existingId)!.session,
        })
      const sessionId = `prepared-${ownerId}${preparationCreates.length ? '-' + (preparationCreates.length + 1) : ''}`
      preparationCreates.push(sessionId)
      createRequests.set(requestKey, sessionId)
      const response = create(Authoring.method.createAuthoringSession.output, {
        session: {
          id: sessionId,
          revision: 1,
          kind: request.kind,
          phase: 'choosing',
          candidateCount: 8,
        },
      })
      preparationSessions.set(sessionId, { ownerId, session: response.session! })
      return response
    })
    rpc(Authoring.method.getAuthoringSession, (request) => {
      calls.push('GetAuthoringSession')
      const stored = preparationSessions.get(request.sessionId)
      if (!stored || stored.ownerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      return create(Authoring.method.getAuthoringSession.output, { session: stored.session })
    })
    rpc(Authoring.method.startAuthoringOperation, (request) => {
      calls.push('StartAuthoringOperation')
      const stored = preparationSessions.get(request.sessionId)
      if (!stored || stored.ownerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      const repeated = preparations.find((entry) => entry.requestKey === request.requestId)
      if (repeated)
        return create(Authoring.method.startAuthoringOperation.output, {
          jobId: 'preparation-job',
          session: stored.session,
        })
      const kind = stored.session.kind
      const body =
        options.preparationBodies?.[preparations.length] ??
        '<write>Prepared writing structure</write>'
      preparations.push({
        kind,
        count: request.candidateCount,
        prompt: request.prompt,
        requestKey: request.requestId,
      })
      const response = create(Authoring.method.startAuthoringOperation.output, {
        jobId: 'preparation-job',
        session: {
          id: request.sessionId,
          revision: 9,
          kind,
          phase: 'choosing',
          candidateCount: request.candidateCount,
          candidates: Array.from({ length: request.candidateCount }, (_, index) => ({
            id: `prepared-candidate-${index}`,
            revision: index + 2,
            name: `Prepared ${index + 1}`,
            description: `Unsaved ${index + 1}`,
            body,
          })),
        },
      })
      stored.session = response.session!
      return response
    })
    rpc(Service.method.estimateWritingTest, (request) => {
      calls.push('EstimateWritingTest')
      if (!request.plan) throw new ConnectError('plan required', Code.InvalidArgument)
      estimates.push(request.plan)
      if (estimatesToRefuse > 0) {
        estimatesToRefuse--
        throw new ConnectError('estimate refused', Code.InvalidArgument)
      }
      return create(Service.method.estimateWritingTest.output, {
        credits: BigInt(request.plan.count * 3),
        quoteKey: `quote-${estimates.length}`,
        expiresAt: '2099-01-01T00:00:00Z',
      })
    })
    rpc(Service.method.startWritingTest, async (request) => {
      calls.push('StartWritingTest')
      admissions.push(request)
      const caller = ownerId
      if (!request.plan) throw new ConnectError('plan required', Code.InvalidArgument)
      if (options.generationGate) await options.generationGate
      admittedPlan = request.plan
      ledger = wireTest(request.plan.count as TestCount)
      ledgerOwnerId = caller
      ledger.factor = request.plan.factor
      ledger.modelStage = request.plan.modelStage
      ledger.sourcePostSlug = request.plan.context?.sourcePostSlug ?? ''
      ledger.targetLanguage = request.plan.context?.targetLanguage ?? contentLanguageToProto('ko')
      ledger.fictional = request.plan.context?.material?.fictional ?? false
      return create(Service.method.startWritingTest.output, { test: ledger })
    })
    rpc(Service.method.getWritingTest, () => {
      calls.push('GetWritingTest')
      if (!ledger || ledgerOwnerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      return create(Service.method.getWritingTest.output, { test: ledger })
    })
    rpc(Service.method.listWritingTests, () => {
      calls.push('ListWritingTests')
      return create(Service.method.listWritingTests.output, {
        tests: ledger && ledgerOwnerId === ownerId ? [ledger] : [],
      })
    })
    rpc(Service.method.decideTestMatch, (request) => {
      calls.push('DecideTestMatch')
      votes.push(request)
      if (!ledger || ledgerOwnerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      if (request.testId !== ledger.id || request.expectedRevision !== ledger.revision)
        throw new ConnectError('stale', Code.FailedPrecondition)
      const match = ledger.matches.find((entry) => entry.id === request.matchId)
      if (
        !match ||
        match.winnerCandidateId ||
        ![match.leftCandidateId, match.rightCandidateId].includes(request.winnerCandidateId)
      )
        throw new ConnectError('invalid match', Code.InvalidArgument)
      match.winnerCandidateId = request.winnerCandidateId
      const parent = ledger.matches.find(
        (entry) => entry.round === match.round + 1 && entry.index === Math.floor(match.index / 2),
      )
      if (parent) {
        if (match.index % 2 === 0) parent.leftCandidateId = request.winnerCandidateId
        else parent.rightCandidateId = request.winnerCandidateId
      }
      ledger.revision++
      if (ledger.matches.every((entry) => !!entry.winnerCandidateId)) {
        ledger.status = WritingTestStatus.COMPLETED
        ledger.revealed = true
        ledger.winnerCandidateId = ledger.matches.at(-1)!.winnerCandidateId
        ledger.candidates.forEach((candidate, index) => {
          candidate.identity = create(WritingTestIdentitySchema, {
            label: `Frozen setting ${index + 1}`,
            source: admittedPlan?.entrants[index] ?? {
              source: {
                case: 'model',
                value: { providerId: 'openrouter', modelId: `model-${index}` },
              },
            },
            synthetic: admittedPlan?.entrants[index].source.case === 'authoringCandidate',
          })
        })
      }
      return create(Service.method.decideTestMatch.output, { test: ledger })
    })
    rpc(Service.method.saveWritingTestWinner, (request) => {
      calls.push('SaveWritingTestWinner')
      publications.push(request)
      if (!ledger || ledgerOwnerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      const existing = ledger.publications.find(
        (receipt) => receipt.requestKey === request.requestKey,
      )
      const publication =
        existing ??
        create(WritingTestPublicationSchema, {
          id: `publication-${ledger.publications.length + 1}`,
          testId: ledger.id,
          winnerCandidateId: ledger.winnerCandidateId,
          action: request.action,
          status: WritingTestPublicationStatus.CONFIRMED,
          requestKey: request.requestKey,
          targetId:
            request.action === WritingTestPublicationAction.ADOPT_MODEL ? 'write' : 'saved-winner',
        })
      if (!existing) {
        ledger.publications.push(publication)
        ledger.revision++
      }
      return create(Service.method.saveWritingTestWinner.output, { test: ledger, publication })
    })
    rpc(Service.method.applyWritingTestOutput, (request) => {
      calls.push('ApplyWritingTestOutput')
      applications.push(request)
      if (!ledger || ledgerOwnerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      const existing = ledger.publications.find(
        (receipt) => receipt.requestKey === request.requestKey,
      )
      const publication =
        existing ??
        create(WritingTestPublicationSchema, {
          id: `application-${ledger.publications.length + 1}`,
          testId: ledger.id,
          winnerCandidateId: ledger.winnerCandidateId,
          action: WritingTestPublicationAction.APPLY_OUTPUT,
          status: WritingTestPublicationStatus.CONFIRMED,
          requestKey: request.requestKey,
          targetId: 'owned-source',
        })
      if (!existing) {
        ledger.publications.push(publication)
        ledger.revision++
      }
      return create(Service.method.applyWritingTestOutput.output, {
        test: ledger,
        publication,
      })
    })
    rpc(Service.method.cancelWritingTest, () => {
      calls.push('CancelWritingTest')
      if (!ledger || ledgerOwnerId !== ownerId) throw new ConnectError('not found', Code.NotFound)
      ledger.status = WritingTestStatus.CANCELLED
      ledger.revealed = true
      ledger.revision++
      return create(Service.method.cancelWritingTest.output, { test: ledger })
    })
  })
  return {
    transport,
    calls,
    estimates,
    admissions,
    votes,
    publications,
    applications,
    preparations,
    preparationEstimates,
    preparationCreates,
    preparationSessions,
    plan,
    setOwner: (id: string) => {
      ownerId = id
    },
    getTest: () => ledger,
    failNextEstimate: () => {
      estimatesToRefuse = 1
    },
  }
}
