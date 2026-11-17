import { afterEach, beforeEach, expect, it } from 'vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, ConnectError, type Transport } from '@connectrpc/connect'
import { initializeI18n } from '@/app/providers/i18n'
import {
  emptySetupProgress,
  readSetupProgress,
  writeSetupProgress,
} from '@/features/complete-setup'
import { renderAppAt, type RenderAppOptions } from '@/test/app'
import { publishAuthoringDraft } from '@/test/authoring-ui'
import { chooseSituations } from '@/test/voice-chat'
import type { FakeTemplatesOptions } from '@/test/templates'
import { createFakeAuthTransport } from '@/test/session'

const OWNER = 'setup-alice'
const READY: RenderAppOptions = {
  user: { id: OWNER },
  firstUseSetup: true,
  voice: { voices: [{ id: 'voice-default', name: '내 말투', made: true }] },
  templates: {
    templates: [{ id: 'post-format', name: '글 구성', body: '<write>오늘의 이야기</write>' }],
  },
  clips: {
    templates: [{ id: 'clip-format', name: '영상 구성', compositionBody: '<clip version="1"/>' }],
  },
}
beforeEach(() => {
  localStorage.clear()
  initializeI18n('ko')
})
afterEach(() => {
  cleanup()
  localStorage.clear()
  initializeI18n('ko')
})
function resumeSetup(resume: 'welcome' | 'voice' | 'post-template' | 'clip-template' = 'welcome') {
  writeSetupProgress(OWNER, { ...emptySetupProgress(), skipped: [], resume })
}
/** Home no longer sends a new account into setup; setup is opened from home's empty kit, at its own
 *  address. */
function mountFresh(extra: RenderAppOptions = {}) {
  return renderAppAt('/setup', {
    user: { id: OWNER },
    firstUseSetup: true,
    voice: { voices: [] },
    ...extra,
  })
}
async function start() {
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '내 작업 공간 준비하기' }))
  return user
}
/** Where a sequence stands, drawn as every guided flow draws it (GuideProgress): one segment per
 *  step, the one on screen marked as the current step. */
function guideStep(name: string) {
  const progress = screen.getByRole('navigation', { name })
  const segments = within(progress).getAllByRole('listitem')
  return {
    total: segments.length,
    current: segments.findIndex((segment) => segment.querySelector('[aria-current="step"]')),
  }
}
const SETUP_PROGRESS = '나만의 작업 공간 준비'
const ANSWER_PROGRESS = '이번 대화의 저장한 답변 수'

it('reads the missing settings without creating content or paid work, and home offers it', async () => {
  const calls: string[] = []
  const { router } = mountFresh({
    voice: { voices: [], calls },
    templates: { calls },
    clips: { calls },
  })
  await screen.findByRole('heading', { name: '처음 한 번, 나답게 준비해요.' })
  expect(router.state.location.pathname).toBe('/setup')
  expect(screen.queryByRole('navigation', { name: '현재 위치' })).not.toBeInTheDocument()
  expect(calls).toContain('ListVoices')
  expect(calls).toContain('ListTemplates')
  expect(calls).toContain('ListVideoTemplates')
  expect(calls.filter((call) => /Create|Start|Analyze|Save/.test(call))).toEqual([])
  // Home shows an empty kit as itself, offering the setup that fills several layers at once.
  await act(() => router.navigate({ to: '/' }))
  expect(await screen.findByRole('link', { name: '한 번에 준비하기' })).toHaveAttribute(
    'href',
    '/setup?restart=false',
  )
})
it('opens home on the kit and 글쓰기 for configured users, with no setup offer', async () => {
  renderAppAt('/', READY)
  expect(
    await within(await screen.findByRole('main')).findByRole('link', { name: '글쓰기' }),
  ).toHaveAttribute('href', '/posts/new')
  expect(screen.queryByRole('heading', { name: /처음 한 번/ })).toBeNull()
  expect(screen.queryByRole('link', { name: '한 번에 준비하기' })).toBeNull()
})
it('does not intercept existing drafts or protected creation deep links', async () => {
  const { router } = renderAppAt('/posts/new', {
    user: { id: OWNER },
    firstUseSetup: true,
    voice: { voices: [] },
  })
  expect(await screen.findByLabelText('제목')).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/posts/new')
  expect(screen.queryByRole('button', { name: '내 작업 공간 준비하기' })).toBeNull()
})
it('distinguishes failed directory reads from missing settings and offers explicit continuation', async () => {
  const user = userEvent.setup()
  mountFresh({ voice: { voices: [], listFails: true } })
  expect(await screen.findByRole('alert')).toHaveTextContent('설정을 확인하지 못했어요')
  expect(screen.queryByRole('button', { name: '내 작업 공간 준비하기' })).toBeNull()
  await user.click(screen.getByRole('button', { name: '먼저 시작하기' }))
  expect(
    await within(await screen.findByRole('main')).findByRole('link', { name: '글쓰기' }),
  ).toBeInTheDocument()
  expect(readSetupProgress(OWNER).completed).toBe(true)
})
function gateProcedure(
  base: Transport,
  procedure: string,
  barrier: Promise<void>,
  fail = false,
): Transport {
  return new Proxy(base, {
    get(target, property) {
      if (property !== 'unary') return Reflect.get(target, property)
      return (...args: unknown[]) => {
        const call = () => Reflect.apply(target.unary, target, args)
        if ((args[0] as { name: string }).name !== procedure) return call()
        return (async () => {
          await barrier
          if (fail) throw new ConnectError('temporarily unavailable', Code.Unavailable)
          return call()
        })()
      }
    },
  })
}
it.each(['pending', 'failed'] as const)(
  'keeps questions available while recommended model preparation is %s',
  async (phase) => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const options: RenderAppOptions = {
      user: { id: OWNER },
      firstUseSetup: true,
      voice: { voices: [] },
    }
    const base = createFakeAuthTransport({ ...options, existingSetup: true })
    renderAppAt('/setup', {
      ...options,
      transport: gateProcedure(
        base,
        'InitializeDefaultSelections',
        phase === 'pending' ? gate : Promise.resolve(),
        phase === 'failed',
      ),
    })
    const user = await start()
    expect(await screen.findByRole('button', { name: '대화하기' })).toBeEnabled()
    expect(screen.queryByRole('heading', { name: '함께 만들 AI를 골라볼까요?' })).toBeNull()
    await user.click(screen.getByRole('button', { name: '대화하기' }))
    await chooseSituations(user)
    expect(await screen.findByLabelText('답')).toBeEnabled()
    await act(async () => release())
  },
)
it('omits made voices and existing templates, showing only the missing clip template after welcome', async () => {
  resumeSetup()
  renderAppAt('/setup', { ...READY, clips: { templates: [] } })
  const user = await start()
  expect(
    await screen.findByRole('heading', { name: '영상에도 나만의 흐름을 담아요.' }),
  ).toBeInTheDocument()
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  await user.click(screen.getByRole('button', { name: '지금은 건너뛰기' }))
  expect(
    await screen.findByRole('heading', { name: '이제, 당신의 이야기를 만들어요.' }),
  ).toBeInTheDocument()
})
// A template question is answered by talking to the AI — the conversation that makes a template
// everywhere else — over the one step bar every question shares: setup's way back and its skip.
it('asks a template question as the AI conversation, whose composer is the one bottom dock', async () => {
  resumeSetup()
  const calls: string[] = []
  renderAppAt('/setup', { ...READY, calls, clips: { templates: [], calls } })
  const user = await start()
  await screen.findByRole('heading', { name: '영상에도 나만의 흐름을 담아요.' })
  expect(await screen.findByRole('log', { name: 'AI와 나눈 대화' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '직접 고치기' })).toBeInTheDocument()
  // No wizard: nothing to pick from a catalogue, and no wizard 다음.
  expect(screen.queryByRole('button', { name: '다음' })).toBeNull()
  // The conversation's composer docks at the bottom; the way back and the skip close the page
  // rather than floating a second dock over it.
  expect(document.querySelectorAll('[data-action-bar="step"]')).toHaveLength(0)
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: '이전' }))
  expect(await screen.findByRole('button', { name: '내 작업 공간 준비하기' })).toBeInTheDocument()
  // Opening the question calls no model.
  expect(calls.filter((call) => /StartAuthoring|EstimateAuthoring/.test(call))).toEqual([])
})
it('defers to home and keeps that browser’s deferral to its own account', async () => {
  const user = userEvent.setup()
  const first = mountFresh()
  await user.click(await screen.findByRole('button', { name: '나중에 설정할게요' }))
  await within(await screen.findByRole('main')).findByRole('link', { name: '글쓰기' })
  expect(first.router.state.location.pathname).toBe('/')
  expect(readSetupProgress(OWNER).completed).toBe(true)
  first.unmount()
  // Another account on the same browser is offered its own setup from its own empty kit.
  renderAppAt('/', { user: { id: 'setup-bob' }, firstUseSetup: true, voice: { voices: [] } })
  expect(await screen.findByRole('link', { name: '한 번에 준비하기' })).toBeInTheDocument()
})
it('skips optional setup without entity writes and finishes at the explicitly chosen creation destination', async () => {
  const calls: string[] = []
  mountFresh({ voice: { voices: [], calls }, templates: { calls }, clips: { calls } })
  const user = await start()
  await screen.findByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })
  // Setup's place among its questions is a segment row, not a bar, and it moves on each skip.
  expect(screen.queryByRole('progressbar')).toBeNull()
  expect(guideStep(SETUP_PROGRESS)).toEqual({ total: 3, current: 0 })
  for (let i = 0; i < 3; i++) {
    await user.click(await screen.findByRole('button', { name: '지금은 건너뛰기' }))
    if (i < 2)
      await waitFor(() => expect(guideStep(SETUP_PROGRESS)).toEqual({ total: 3, current: i + 1 }))
  }
  await user.click(await screen.findByRole('button', { name: '첫 클립 만들기' }))
  await screen.findByLabelText('클립 제목')
  expect(calls.filter((call) => /Create|Start|Analyze|Save/.test(call))).toEqual([])
  expect(readSetupProgress(OWNER)).toMatchObject({ completed: true, target: '/clips/new' })
})
it('opens a creation reached from its finish without offering again what setup just asked', async () => {
  const { router } = mountFresh({ creationOffers: true })
  const user = await start()
  await screen.findByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })
  for (let i = 0; i < 3; i++)
    await user.click(await screen.findByRole('button', { name: '지금은 건너뛰기' }))
  await user.click(await screen.findByRole('button', { name: '첫 글 작성하기' }))

  await screen.findByLabelText('제목')
  expect(router.state.location.pathname).toBe('/posts/new')
  expect(router.state.location.search).toMatchObject({ from: 'setup' })
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
it('finishes at home with no creation handoff', async () => {
  const { router } = mountFresh()
  const user = await start()
  await screen.findByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })
  for (let i = 0; i < 3; i++)
    await user.click(await screen.findByRole('button', { name: '지금은 건너뛰기' }))
  await user.click(await screen.findByRole('button', { name: '만들러 가기' }))

  await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  expect(router.state.location.search).not.toHaveProperty('from')
})
it('resumes an unfinished voice rather than creating another one', async () => {
  resumeSetup('voice')
  const creates: Array<{ name: string }> = []
  mountFresh({
    voice: { voices: [{ id: 'voice-default', name: '진행 중', made: false }], creates },
  })
  expect(
    await screen.findByRole('heading', { name: '글에 나의 말투를 담아볼까요?' }),
  ).toBeInTheDocument()
  await chooseSituations(userEvent.setup())
  expect(await screen.findByLabelText('답')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  expect(creates).toEqual([])
})
it('creates a personal voice once with an automatic name and holds the funnel while the request is pending', async () => {
  resumeSetup('voice')
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const calls: string[] = [],
    creates: Array<{ name: string }> = []
  mountFresh({ voice: { voices: [], calls, creates, createGate: gate } })
  const user = userEvent.setup()
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  await user.dblClick(await screen.findByRole('button', { name: '대화하기' }))
  expect(calls.filter((call) => call === 'CreateVoice')).toHaveLength(1)
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeDisabled()
  await act(async () => release())
  await chooseSituations(user)
  expect(await screen.findByLabelText('답')).toBeInTheDocument()
  expect(creates).toEqual([{ name: '나의 말투' }])
  expect(screen.getByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })).toBeInTheDocument()
  expect(calls).not.toContain('AnalyzeVoice')
})
it('retains the failed step and returns to all three methods on explicit Back without another create', async () => {
  resumeSetup('voice')
  const calls: string[] = []
  mountFresh({ voice: { voices: [], createFails: true, calls } })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '대화하기' }))
  await screen.findByText(/연결|네트워크|요청을 마치지/)
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  expect(screen.getByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '대화하기' })).toBeNull()
  expect(screen.getByRole('button', { name: '다시 확인하고 시도하기' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: '이전' }))
  expect(screen.getByRole('button', { name: '대화하기' })).toBeEnabled()
  expect(screen.getByRole('button', { name: '내가 쓴 글로 말투 알려 주기' })).toBeEnabled()
  expect(screen.getByRole('button', { name: 'AI가 추천한 말투에서 고르기' })).toBeEnabled()
  expect(calls.filter((call) => call === 'CreateVoice')).toHaveLength(1)
  expect(calls).not.toContain('AnalyzeVoice')
})
/** Writes a template by hand in the setup conversation's secondary path, then saves it with the
 *  conversation's own 저장. No model is called on the way. */
async function writePostTemplate(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(await screen.findByRole('button', { name: '직접 고치기' }))
  await user.type(await screen.findByLabelText('이름'), name)
  const body = screen.getByLabelText('내용')
  await user.clear(body)
  await user.type(body, '방문 계기{enter}<write>이곳을 찾게 된 이유</write>')
}
it('keeps setup on the post step while a hand-written template is refused, with its words kept', async () => {
  resumeSetup('post-template')
  const calls: string[] = []
  mountFresh({ voice: READY.voice, calls, templates: { calls, createFails: true } })
  const user = userEvent.setup()
  await screen.findByRole('heading', { name: '자주 쓰는 글의 구성을 정해요.' })
  await writePostTemplate(user, '일상 글')
  await publishAuthoringDraft(user)
  await screen.findByText(/연결|네트워크|요청을 마치지|저장하지 못했/)
  expect(screen.getByRole('heading', { name: '자주 쓰는 글의 구성을 정해요.' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeEnabled()
  expect(calls.filter((call) => /StartAuthoring|EstimateAuthoring/.test(call))).toEqual([])
})
it('recovers a running voice analysis after reload without starting another paid job', async () => {
  resumeSetup('voice')
  const calls: string[] = []
  mountFresh({
    voice: {
      voices: [{ id: 'voice-default', name: '학습 중', made: false }],
      activeJobId: 'voice-job',
      analysisAfterAnalysis: '차분해요',
      calls,
    },
    jobs: { jobs: [{ id: 'voice-job', kind: 'analyze_voice', status: 'done', stage: 'analyze' }] },
  })
  expect(await screen.findByText('내 말투가 준비됐어요')).toBeInTheDocument()
  expect(calls).not.toContain('AnalyzeVoice')
  const user = userEvent.setup()
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '이 말투를 내 글에 사용하기' })).toBeEnabled(),
  )
  await user.click(screen.getByRole('button', { name: '이 말투를 내 글에 사용하기' }))
  expect(
    await screen.findByRole('heading', { name: '자주 쓰는 글의 구성을 정해요.' }),
  ).toBeInTheDocument()
  expect(calls).toContain('SetDefaultVoice')
})
it('never advances while a recovered analysis remains active', async () => {
  resumeSetup('voice')
  mountFresh({
    voice: {
      voices: [{ id: 'voice-default', name: '학습 중', made: false }],
      activeJobId: 'voice-job',
    },
    jobs: {
      jobs: [{ id: 'voice-job', kind: 'analyze_voice', status: 'running', stage: 'analyze' }],
    },
  })
  expect(await screen.findByRole('button', { name: '지금은 건너뛰기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '이전' })).toBeDisabled()
})
it('offers setup again only on an explicit settings entry after completion', async () => {
  writeSetupProgress(OWNER, { ...emptySetupProgress(), completed: true })
  const { router } = renderAppAt('/settings', {
    user: { id: OWNER },
    firstUseSetup: true,
    voice: { voices: [] },
  })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('link', { name: '처음 설정 다시 살펴보기' }))
  expect(
    await screen.findByRole('heading', { name: '처음 한 번, 나답게 준비해요.' }),
  ).toBeInTheDocument()
  expect(router.state.location.search.restart).toBe(true)
})

it('saves a post template once from the conversation and advances from its confirmed save', async () => {
  resumeSetup('post-template')
  const creates: FakeTemplatesOptions['creates'] = []
  mountFresh({ voice: READY.voice, templates: { creates } })
  const user = userEvent.setup()
  await screen.findByRole('heading', { name: '자주 쓰는 글의 구성을 정해요.' })
  await writePostTemplate(user, '내 글 구성')
  await publishAuthoringDraft(user)
  expect(
    await screen.findByRole('heading', { name: '영상에도 나만의 흐름을 담아요.' }),
  ).toBeInTheDocument()
  expect(creates).toHaveLength(1)
  expect(creates![0]).toMatchObject({ name: '내 글 구성' })
})
it('offers completion after the video question and lands on home', async () => {
  resumeSetup('clip-template')
  const { router } = renderAppAt('/setup', { ...READY, clips: { templates: [] } })
  const user = userEvent.setup()
  await screen.findByRole('heading', { name: '영상에도 나만의 흐름을 담아요.' })
  await user.click(screen.getByRole('button', { name: '지금은 건너뛰기' }))
  expect(
    await screen.findByRole('heading', { name: '이제, 당신의 이야기를 만들어요.' }),
  ).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '만들러 가기' }))
  expect(
    await within(screen.getByRole('main')).findByRole('link', { name: '글쓰기' }),
  ).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
})

it('starts with writing identity while available recommended models are prepared automatically', async () => {
  const calls: string[] = []
  mountFresh({
    providers: {
      calls,
      models: [{ providerId: 'openrouter', modelId: 'creative', label: 'Creative', vision: true }],
    },
  })
  await start()
  expect(await screen.findByRole('button', { name: '대화하기' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'AI가 추천한 말투에서 고르기' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '내가 쓴 글로 말투 알려 주기' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '이 자료로 내 말투 만들기' })).toBeNull()
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  await waitFor(() => expect(calls).toContain('InitializeDefaultSelections'))
  expect(calls).not.toContain('SaveSelection')
  expect(calls.filter((call) => /^(Create|Start|Analyze)/.test(call))).toEqual([])
})
it('saves ten chat replies through setup and waits for an explicit analysis action on the recommended model', async () => {
  resumeSetup('voice')
  const calls: string[] = [],
    answers: Array<{ promptKey: string; body: string; uploadId: string }> = [],
    analyses: Array<{ voiceId: string; model: string }> = []
  mountFresh({
    calls,
    voice: { voices: [{ id: 'voice-default', name: '진행 중', made: false }], answers, analyses },
    providers: {
      models: [{ providerId: 'stub', modelId: 'recommended', label: '추천 AI', vision: true }],
    },
    jobs: {
      jobs: [{ id: 'voice-job', kind: 'analyze_voice', status: 'running', stage: 'analyze' }],
    },
  })
  const user = userEvent.setup()
  await chooseSituations(user)
  await screen.findByLabelText('답')
  expect(screen.queryByRole('navigation', { name: '현재 위치' })).not.toBeInTheDocument()
  // One segment per answer the chat asks for; the one being answered now is current.
  expect(guideStep(ANSWER_PROGRESS)).toEqual({ total: 10, current: 0 })
  for (let index = 0; index < 10; index++) {
    await user.type(screen.getByLabelText('답'), '친구에게 평소처럼 편하게 이야기했어요.')
    await user.click(screen.getByRole('button', { name: '보내기' }))
    if (index < 9)
      await waitFor(() =>
        expect(guideStep(ANSWER_PROGRESS)).toEqual({ total: 10, current: index + 1 }),
      )
  }
  expect(await screen.findByText('열 개의 답변이 모였어요.')).toBeInTheDocument()
  expect(answers).toHaveLength(10)
  expect(new Set(answers.map((answer) => answer.promptKey)).size).toBe(10)
  expect(analyses).toEqual([])
  expect(calls).not.toContain('AnalyzeVoice')
  await user.click(screen.getByRole('button', { name: '저장한 답변 확인하기' }))
  expect(analyses).toEqual([])
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '이 자료로 내 말투 만들기' })).toBeEnabled(),
  )
  await user.click(screen.getByRole('button', { name: '이 자료로 내 말투 만들기' }))
  expect(analyses).toEqual([])
  const quote = await screen.findByRole('dialog', { name: '이 자료로 말투를 분석할까요?' })
  await within(quote).findByText(/예상 .*크레딧/)
  const confirm = within(quote).getByRole('button', { name: '분석 시작' })
  await waitFor(() => expect(confirm).toBeEnabled())
  await user.click(confirm)
  await waitFor(() =>
    expect(analyses).toEqual([{ voiceId: 'voice-default', model: 'stub/recommended' }]),
  )
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeDisabled()
  expect(screen.getByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })).toBeInTheDocument()
})
it('guards parent navigation while an inline answer save is pending', async () => {
  resumeSetup('voice')
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const options: RenderAppOptions = {
    user: { id: OWNER },
    firstUseSetup: true,
    voice: { voices: [{ id: 'voice-default', name: '진행 중', made: false }] },
  }
  const base = createFakeAuthTransport({ ...options, existingSetup: true })
  renderAppAt('/setup', { ...options, transport: gateProcedure(base, 'AnswerVoicePrompt', gate) })
  const user = userEvent.setup()
  await chooseSituations(user)
  await user.type(await screen.findByLabelText('답'), '저장 중에도 입력한 내용은 그대로예요.')
  await user.click(screen.getByRole('button', { name: '보내기' }))
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '이전' })).toBeDisabled()
  expect(screen.getByLabelText('답')).toHaveValue('저장 중에도 입력한 내용은 그대로예요.')
  expect(guideStep(ANSWER_PROGRESS).current).toBe(0)
  await act(async () => release())
  await waitFor(() => expect(guideStep(ANSWER_PROGRESS).current).toBe(1))
})
