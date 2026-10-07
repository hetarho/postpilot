import { createRoot } from 'react-dom/client'
import { createRouter, createMemoryHistory, RouterProvider } from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TransportProvider } from '@connectrpc/connect-query'
import { Code, type Transport } from '@connectrpc/connect'
import { routeTree } from '@/app/routes/router'
import { initializeI18n } from '@/app/providers/i18n'
import { ThemeProvider, bootstrapTheme } from '@/app/providers/theme'
import { createFakeAuthTransport, type FakeAuthOptions } from '@/test/session'
import { connectAppError } from '@/test/app-error'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { spokenVoiceFixture } from '@/test/spoken-voices'
import { clipTimelineFixture } from '@/test/clip-editing'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import {
  Stage,
  ProtoPlan,
  WritingTestService,
  SpokenVoiceService,
  SpokenVoiceGenerationService,
  SpeechProfileService,
  WritingTestStatus,
} from '@/shared/api'
import '@/app/styles/index.css'

const params = new URLSearchParams(location.search)
const at = params.get('at') ?? '/tests'
const scenario = params.get('state') ?? 'populated'
const publicRoute = /^\/(login|signup|forgot-password|reset-password|verify-email)(?:[/?]|$)/.test(
  at,
)
initializeI18n('ko')
const themeSnapshot = bootstrapTheme()
const ownerId = 'alice'
const procedures: string[] = []
const tests = createWritingTestStudioFixture({ readableTest: scenario !== 'empty' })
tests.setOwner(ownerId)
const spoken = spokenVoiceFixture(
  scenario === 'empty'
    ? {}
    : {
        preset: scenario === 'spoken-candidates' ? 'candidates' : 'confirmed',
        operationState: scenario === 'spoken-working' ? 'running' : undefined,
      },
)
const options: FakeAuthOptions = {
  existingSetup: scenario !== 'setup',
  user: publicRoute
    ? undefined
    : {
        id: ownerId,
        email: 'alice@example.test',
        emailVerified: true,
        plan: scenario === 'checkout' ? ProtoPlan.BASIC : ProtoPlan.MASTER,
      },
  resetPasswordFails: scenario === 'invalid-reset',
  verificationFails: scenario === 'invalid-verification',
  providers: {
    models: Array.from({ length: 16 }, (_, i) => ({
      providerId: 'openrouter',
      modelId: `model-${i}`,
      label: `Model ${i + 1}`,
      vision: true,
      structuredOutput: true,
      affordable: true,
      stages: [Stage.WRITE, Stage.OBSERVE, Stage.ANALYZE],
    })),
    selections: [Stage.WRITE, Stage.OBSERVE, Stage.ANALYZE].map((stage) => ({
      stage,
      providerId: 'openrouter',
      modelId: 'model-0',
    })),
  },
  posts: {
    posts: [
      {
        slug: 'review',
        title: '공원에서 작은 카페까지 이어지는 동네 산책과 일상 경험 기록',
        memo: '공원과 작은 카페를 방문했습니다.',
        status: 'review',
        content: POST_CONTENT_FIXTURE,
        contentRevision: 1n,
      },
    ],
  },
  templates: {
    templates: [
      {
        id: 'template',
        name: '산책과 일상 경험을 차분하게 정리하는 아주 긴 한국어 구성 이름',
        body: '<write>경험한 내용을 차분하게 정리해 주세요.</write>',
      },
    ],
  },
  guidelines: {
    guidelines: [
      {
        id: 'guideline',
        title: '개인 경험을 과장 없이 담아내는 아주 긴 한국어 작문 지침',
        text: '내 경험을 담고 과장된 표현은 피해주세요.',
        scope: 'global',
      },
    ],
  },
  voice: {
    voices: [
      { id: 'voice-default', name: '나의 말투', made: scenario !== 'setup', isDefault: true },
    ],
    samples: [
      { id: 'sample', label: '산책 기록', body: '천천히 걷고 풍경을 보았습니다.'.repeat(25) },
    ],
  },
  experiments: { history: [{ id: 'record', stage: Stage.WRITE, postSlug: 'review' }] },
  vouchers: { vouchers: [{ token: 'gift', message: '당신의 일상 이야기를 담아 보세요.' }] },
  billing: { populated: scenario !== 'empty', paymentMethod: true, subscription: true },
  plans: { accounts: [{ id: 'alice@example.test', plan: ProtoPlan.MASTER }] },
  memories: {
    memories: [
      {
        id: 'memory',
        text: '지난 경험을 기록한 오래도록 남겨두고 싶은 아주 긴 한국어 기억입니다.',
      },
    ],
  },
  clips: {
    templates: [
      { id: 'video-template', name: '영상 구성', compositionBody: '<clip version="1"/>' },
    ],
    projects: [
      {
        id: 'review',
        title: '공원과 카페를 방문한 산책 하루의 아주 긴 한국어 영상 제목',
        videoTemplateId: 'video-template',
        ratio: 'vertical',
        targetDurationMs: 19800,
        disclosure: 'ad',
        editPlanRevision: 1,
        renderedPlanRevision: 1,
        editing: clipTimelineFixture(),
        result: {
          contentType: 'video/mp4',
          bytes: 5,
          durationMs: 19800,
          createdAt: '2026-10-07T00:00:00Z',
          viewUrl: 'https://private.test/old',
          downloadUrl: 'https://private.test/download',
        },
      },
    ],
  },
}
if (scenario === 'empty') {
  options.posts = { posts: [] }
  options.templates = { templates: [] }
  options.guidelines = { guidelines: [] }
  options.voice = { voices: [] }
  options.experiments = { history: [] }
  options.memories = { memories: [] }
  options.vouchers = { vouchers: [] }
  options.clips = { templates: [], projects: [] }
  options.billing = { populated: false, paymentMethod: false, subscription: false }
}
if (scenario === 'catalog-populated')
  options.modelCatalog = {
    entries: Array.from({ length: 24 }, (_, i) => ({
      modelId: `provider-${i % 3}/mobile-model-${i}`,
      label: `모바일에서 읽는 긴 모델 이름 ${i + 1}`,
      vision: true,
      structuredOutput: true,
      curated: true,
      purposes: ['photo-analysis', 'writing'],
      level: {
        'photo-analysis': ['value', 'balanced', 'premium', 'top'][i % 4],
        writing: ['value', 'balanced', 'premium', 'top'][i % 4],
      },
      sourceCreatedAt: BigInt(i),
    })),
  }
if (scenario === 'checkout') options.billing = { paymentMethod: true }
if (scenario === 'finalized') {
  options.posts!.posts![0].status = 'finalized'
  options.clips!.projects![0].result!.id = 'render-1'
  options.clips!.projects![0].finalized = {
    at: '2026-10-07T00:00:00Z',
    planRevision: 1,
    resultId: 'render-1',
  }
}
if (scenario === 'draft') {
  options.posts!.posts![0] = { ...options.posts!.posts![0], status: 'draft', content: undefined }
  options.clips!.projects![0] = {
    ...options.clips!.projects![0],
    editing: undefined,
    result: undefined,
    editPlanRevision: 0,
    renderedPlanRevision: 0,
  }
}
if (scenario === 'working' || scenario === 'failed') {
  const job = {
    id: 'audit-job',
    kind: 'generate',
    status: scenario === 'working' ? 'running' : 'failed',
    stage: 'write',
    progressDone: 1,
    progressTotal: 2,
    failureReason: scenario === 'failed' ? ('MODEL_UNAVAILABLE' as const) : undefined,
  }
  options.jobs = { jobs: [job] }
  options.posts!.posts![0] = {
    ...options.posts!.posts![0],
    activeJob: job,
    latestOrdinaryFailure: scenario === 'failed' ? job : undefined,
  }
  options.clips!.projects![0] = {
    ...options.clips!.projects![0],
    latestJob: { ...job, clipProjectId: 'review', kind: 'clip_render' },
  }
  const ledger = tests.getTest()
  if (ledger) ledger.jobId = 'audit-job'
  if (ledger)
    ledger.status = scenario === 'working' ? WritingTestStatus.RUNNING : WritingTestStatus.FAILED
}
const base = createFakeAuthTransport(options)

const transport: Transport = new Proxy(base, {
  get(target, property) {
    if (property !== 'unary') return Reflect.get(target, property)
    return async (...args: unknown[]) => {
      const method = args[0] as { name: string; parent: { typeName: string } }
      procedures.push(method.name)
      if (scenario === 'error' && method.name !== 'GetMe' && /^(List|Get)/.test(method.name))
        throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
      if (method.parent.typeName === WritingTestService.typeName)
        return Reflect.apply(tests.transport.unary, tests.transport, args)
      if (
        [
          SpokenVoiceService.typeName,
          SpokenVoiceGenerationService.typeName,
          SpeechProfileService.typeName,
        ].includes(method.parent.typeName)
      )
        return Reflect.apply(spoken.transport.unary, spoken.transport, args)
      return Reflect.apply(target.unary, target, args)
    }
  },
})
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
const router = createRouter({
  routeTree,
  history: createMemoryHistory({ initialEntries: [at] }),
  context: { queryClient, transport },
})
;(window as unknown as { __mobileAudit: unknown }).__mobileAudit = {
  router,
  fixtureKey: `${scenario}:${publicRoute}`,
  procedures,
  tests,
  spoken,
  queryClient,
  transport,
  routeInventory: Object.values(router.routesById).map((route) => ({
    id: route.id,
    fullPath: route.fullPath,
    path: route.path,
    hasComponent: Boolean(route.options.component),
    hasBeforeLoad: Boolean(route.options.beforeLoad),
    parentId: route.parentRoute?.id,
  })),
}
createRoot(document.getElementById('root')!).render(
  <ThemeProvider initialSnapshot={themeSnapshot}>
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>
  </ThemeProvider>,
)
