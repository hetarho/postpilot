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
function mountFresh(extra: RenderAppOptions = {}) {
  return renderAppAt('/', {
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

it('offers setup after missing settings were read without creating content or paid work', async () => {
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
})
it('shows the two-choice launch for configured users and acknowledges that browser', async () => {
  renderAppAt('/', READY)
  expect(await screen.findByRole('link', { name: /새 글 작성하기/ })).toHaveAttribute(
    'href',
    '/posts/new',
  )
  expect(screen.queryByRole('heading', { name: /처음 한 번/ })).toBeNull()
  expect(readSetupProgress(OWNER).completed).toBe(true)
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
  expect(await screen.findByRole('link', { name: /새 글 작성하기/ })).toBeInTheDocument()
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
    expect(await screen.findByRole('button', { name: '질문 10개로 내 말투 찾기' })).toBeEnabled()
    expect(screen.queryByRole('heading', { name: '함께 만들 AI를 골라볼까요?' })).toBeNull()
    await user.click(screen.getByRole('button', { name: '질문 10개로 내 말투 찾기' }))
    expect(await screen.findByLabelText('답')).toBeEnabled()
    await act(async () => release())
  },
)
it('omits made voices and existing templates, showing only the missing clip template after welcome', async () => {
  resumeSetup()
  renderAppAt('/', { ...READY, clips: { templates: [] } })
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
it('defers once, survives reload and does not share the acknowledgement with another account', async () => {
  const user = userEvent.setup()
  const first = mountFresh()
  await user.click(await screen.findByRole('button', { name: '나중에 설정할게요' }))
  await screen.findByRole('link', { name: /새 글 작성하기/ })
  first.unmount()
  const second = mountFresh()
  await screen.findByRole('link', { name: /새 글 작성하기/ })
  expect(second.router.state.location.pathname).toBe('/')
  second.unmount()
  renderAppAt('/', { user: { id: 'setup-bob' }, firstUseSetup: true, voice: { voices: [] } })
  expect(
    await screen.findByRole('heading', { name: '처음 한 번, 나답게 준비해요.' }),
  ).toBeInTheDocument()
})
it('skips optional setup without entity writes and finishes at the explicitly chosen creation destination', async () => {
  const calls: string[] = []
  mountFresh({ voice: { voices: [], calls }, templates: { calls }, clips: { calls } })
  const user = await start()
  await screen.findByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })
  for (let i = 0; i < 3; i++)
    await user.click(await screen.findByRole('button', { name: '지금은 건너뛰기' }))
  await user.click(await screen.findByRole('button', { name: '첫 클립 만들기' }))
  await screen.findByLabelText('클립 제목')
  expect(calls.filter((call) => /Create|Start|Analyze|Save/.test(call))).toEqual([])
  expect(readSetupProgress(OWNER)).toMatchObject({ completed: true, target: '/clips/new' })
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
  await user.dblClick(await screen.findByRole('button', { name: '질문 10개로 내 말투 찾기' }))
  expect(calls.filter((call) => call === 'CreateVoice')).toHaveLength(1)
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeDisabled()
  await act(async () => release())
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
  await user.click(await screen.findByRole('button', { name: '질문 10개로 내 말투 찾기' }))
  await screen.findByText(/연결|네트워크|요청을 마치지/)
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  expect(screen.getByRole('heading', { name: '글에 나의 말투를 담아볼까요?' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '질문 10개로 내 말투 찾기' })).toBeNull()
  expect(screen.getByRole('button', { name: '다시 확인하고 시도하기' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: '이전' }))
  expect(screen.getByRole('button', { name: '질문 10개로 내 말투 찾기' })).toBeEnabled()
  expect(screen.getByRole('button', { name: '내가 쓴 글로 말투 알려 주기' })).toBeEnabled()
  expect(screen.getByRole('button', { name: 'AI가 추천한 말투에서 고르기' })).toBeEnabled()
  expect(calls.filter((call) => call === 'CreateVoice')).toHaveLength(1)
  expect(calls).not.toContain('AnalyzeVoice')
})
it('authors and saves a post template once before advancing, retaining content on a server refusal', async () => {
  resumeSetup('post-template')
  const creates: FakeTemplatesOptions['creates'] = []
  const calls: string[] = []
  mountFresh({ voice: READY.voice, templates: { creates, calls, createFails: true } })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '직접 편집' }))
  await user.type(await screen.findByLabelText('템플릿 이름'), '일상 글')
  await user.click(
    within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
      name: /^AI가 쓰는 글/,
    }),
  )
  const text = await screen.findByLabelText('이 자리에 오는 것')
  await user.type(text, '오늘 경험을 자연스럽게 쓴다')
  await user.click(screen.getByRole('button', { name: '저장하고 계속' }))
  await waitFor(() => expect(calls).toContain('CreateTemplate'))
  expect(screen.getByLabelText('템플릿 이름')).toHaveValue('일상 글')
  expect(screen.getByRole('heading', { name: '자주 쓰는 글의 구성을 정해요.' })).toBeInTheDocument()
  expect(text).toHaveValue('오늘 경험을 자연스럽게 쓴다')
  expect(creates).toEqual([])
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

it('creates a valid post template exactly once and advances from its confirmed saved identity', async () => {
  resumeSetup('post-template')
  const creates: FakeTemplatesOptions['creates'] = []
  const first = mountFresh({ voice: READY.voice, templates: { creates } })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '직접 편집' }))
  await user.type(await screen.findByLabelText('템플릿 이름'), '내 글 구성')
  await user.click(
    within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
      name: /^AI가 쓰는 글/,
    }),
  )
  await user.type(await screen.findByLabelText('이 자리에 오는 것'), '오늘 경험을 쓴다')
  await user.dblClick(screen.getByRole('button', { name: '저장하고 계속' }))
  expect(
    await screen.findByRole('heading', { name: '영상에도 나만의 흐름을 담아요.' }),
  ).toBeInTheDocument()
  expect(creates).toHaveLength(1)
  expect(creates![0]).toMatchObject({ name: '내 글 구성', body: '<write>오늘 경험을 쓴다</write>' })
  first.unmount()
  resumeSetup()
  renderAppAt('/', {
    ...READY,
    templates: {
      templates: [{ id: 'saved-post', name: creates![0]!.name, body: creates![0]!.body }],
    },
    clips: { templates: [] },
  })
  const nextUser = await start()
  await nextUser.click(screen.getByRole('button', { name: '지금은 건너뛰기' }))
  expect(
    await screen.findByRole('heading', { name: '이제, 당신의 이야기를 만들어요.' }),
  ).toBeInTheDocument()
})
it('saves a clip template once before offering completion', async () => {
  resumeSetup('clip-template')
  const calls: string[] = []
  const { router } = renderAppAt('/', { ...READY, clips: { templates: [], calls } })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '직접 편집' }))
  await user.type(await screen.findByLabelText('템플릿 이름'), '내 영상 구성')
  await user.dblClick(screen.getByRole('button', { name: '저장하고 계속' }))
  expect(
    await screen.findByRole('heading', { name: '이제, 당신의 이야기를 만들어요.' }),
  ).toBeInTheDocument()
  expect(calls.filter((call) => call === 'CreateVideoTemplate')).toHaveLength(1)
  await user.click(screen.getByRole('button', { name: '만들러 가기' }))
  expect(await screen.findByRole('link', { name: /새 클립 만들기/ })).toBeInTheDocument()
  expect(router.state.location.pathname).toBe('/')
})
it('retains a clip template after a failed save', async () => {
  resumeSetup('clip-template')
  const calls: string[] = []
  renderAppAt('/', { ...READY, clips: { templates: [], calls, saveFails: true } })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '직접 편집' }))
  await user.type(await screen.findByLabelText('템플릿 이름'), '내 영상 구성')
  await user.click(screen.getByRole('button', { name: '저장하고 계속' }))
  await waitFor(() => expect(calls).toContain('CreateVideoTemplate'))
  await waitFor(() => expect(screen.getByRole('button', { name: '저장하고 계속' })).toBeEnabled())
  expect(screen.getByLabelText('템플릿 이름')).toHaveValue('내 영상 구성')
  expect(
    screen.getByRole('heading', { name: '영상에도 나만의 흐름을 담아요.' }),
  ).toBeInTheDocument()
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
  expect(
    await screen.findByRole('button', { name: '질문 10개로 내 말투 찾기' }),
  ).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'AI가 추천한 말투에서 고르기' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '내가 쓴 글로 말투 알려 주기' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '이 자료로 내 말투 만들기' })).toBeNull()
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(screen.queryByLabelText('말투 이름')).toBeNull()
  await waitFor(() => expect(calls).toContain('InitializeDefaultSelections'))
  expect(calls).not.toContain('SaveSelection')
  expect(calls.filter((call) => /^(Create|Start|Analyze)/.test(call))).toEqual([])
})
it('saves ten short answers through setup and waits for an explicit analysis action on the recommended model', async () => {
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
  await screen.findByLabelText('답')
  expect(screen.queryByRole('navigation', { name: '현재 위치' })).not.toBeInTheDocument()
  for (let index = 0; index < 10; index++) {
    await user.type(screen.getByLabelText('답'), '친구에게 평소처럼 편하게 이야기했어요.')
    await user.click(screen.getByRole('button', { name: '답하기' }))
    if (index < 9)
      await waitFor(() =>
        expect(
          screen.getByRole('progressbar', { name: '이번 질문의 저장한 답변 수' }),
        ).toHaveAttribute('aria-valuenow', String(index + 1)),
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
  await user.type(await screen.findByLabelText('답'), '저장 중에도 입력한 내용은 그대로예요.')
  await user.click(screen.getByRole('button', { name: '답하기' }))
  expect(screen.getByRole('button', { name: '지금은 건너뛰기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '이전' })).toBeDisabled()
  expect(screen.getByLabelText('답')).toHaveValue('저장 중에도 입력한 내용은 그대로예요.')
  await act(async () => release())
  await waitFor(() =>
    expect(screen.getByRole('progressbar', { name: '이번 질문의 저장한 답변 수' })).toHaveAttribute(
      'aria-valuenow',
      '1',
    ),
  )
})
