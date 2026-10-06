import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import {
  createRootRoute,
  createRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, within, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { authoringSessionQueryKey } from '@/entities/ai-authoring'
import { initializeI18n } from '@/app/providers/i18n'
import {
  ConfigurationAuthoringService as Service,
  GenerationService,
  ProtoConfigurationKind,
  Stage,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { registerProviderService } from '@/test/providers'
import { connectAppError } from '@/test/app-error'
import { AuthoringEditor, type AuthoringEditorProps } from './AuthoringEditor'
import { AuthoringPreview } from './AuthoringPreview'

type Wire = NonNullable<ReturnType<typeof makeSession>>
function makeSession(patch: object = {}) {
  const fields = JSON.parse(
    JSON.stringify(patch, (key, value) => (key === '$typeName' ? undefined : value)),
  )
  return create(Service.method.getAuthoringSession.output, {
    session: {
      id: 'session',
      kind: ProtoConfigurationKind.POST_GUIDELINE,
      revision: 1,
      phase: 'choosing',
      ...fields,
    },
  }).session!
}
function server(
  options: {
    initial?: Wire
    hold?: boolean
    uncertainStart?: boolean
    saveConflict?: boolean
    saveGate?: Promise<void>
    uncertainSave?: boolean
  } = {},
) {
  let current = options.initial
  let latestCalls = 0
  const calls: string[] = []
  const starts: Array<{
    requestId: string
    expectedRevision: number
    prompt: string
    model: string
  }> = []
  const estimates: Array<{ sessionId: string; model: string }> = []
  const defaults: boolean[] = []
  const accepted = new Map<string, Wire>()
  let uncertain = true
  let publicationUnknown = true
  const candidates = Array.from({ length: 8 }, (_, i) => ({
    id: `candidate-${i}`,
    name: `제안 ${i + 1}`,
    description: '과장 없이 편하게 설명해요',
    body: `실제 경험을 바탕으로 ${i + 1}번째 방향으로 써 주세요.`,
    titleArea: '',
  }))
  const finish = () => {
    if (!current?.activeJobId) return
    if (current.phase === 'generating')
      current = makeSession({
        ...current,
        revision: current.revision + 1,
        phase: 'choosing',
        activeJobId: '',
        candidates,
      })
    else
      current = makeSession({
        ...current,
        revision: current.revision + 1,
        phase: 'editing',
        activeJobId: '',
        selected: { ...current.selected!, body: '짧게 다듬은 지침이에요.' },
        turns: [
          ...current.turns,
          {
            id: 'turn',
            request: current.pendingRequest,
            reply: '조금 더 짧게 다듬었어요.',
            jobId: 'job',
            status: 'done',
          },
        ],
      })
  }
  const transport = createRouterTransport((router) => {
    registerProviderService(router, {
      models: [
        { providerId: 'stub', modelId: 'recommended', label: '추천AI', stages: [Stage.WRITE] },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'stub', modelId: 'recommended' }],
    })
    const { rpc } = router
    rpc(Service.method.getLatestAuthoringSession, () => {
      calls.push('GetLatest')
      latestCalls++
      return create(Service.method.getLatestAuthoringSession.output, { session: current })
    })
    rpc(Service.method.getAuthoringSession, () => {
      calls.push('Get')
      return create(Service.method.getAuthoringSession.output, { session: current })
    })
    rpc(Service.method.createAuthoringSession, (request) => {
      calls.push('Create')
      current = current ?? makeSession({ kind: request.kind, targetId: request.targetId })
      return create(Service.method.createAuthoringSession.output, { session: current })
    })
    rpc(Service.method.estimateAuthoringOperation, (request) => {
      calls.push('Estimate')
      estimates.push({ sessionId: request.sessionId, model: request.writeModel?.modelId ?? '' })
      return create(Service.method.estimateAuthoringOperation.output, { free: false, credits: 5n })
    })
    rpc(Service.method.startAuthoringOperation, (request) => {
      calls.push('Start')
      starts.push({
        requestId: request.requestId,
        expectedRevision: request.expectedRevision,
        prompt: request.prompt,
        model: request.writeModel?.modelId ?? '',
      })
      if (options.uncertainStart && uncertain) {
        uncertain = false
        throw new ConnectError('unknown delivery', Code.Unavailable)
      }
      if (!accepted.has(request.requestId)) {
        current = makeSession({
          ...current!,
          revision: current!.revision + 1,
          phase: request.mode === 1 ? 'generating' : 'refining',
          activeJobId: 'job',
          pendingRequest: request.prompt,
        })
        accepted.set(request.requestId, current)
      }
      return create(Service.method.startAuthoringOperation.output, {
        jobId: 'job',
        session: accepted.get(request.requestId),
      })
    })
    rpc(Service.method.selectAuthoringCandidate, (request) => {
      calls.push('Select')
      current = makeSession({
        ...current!,
        revision: current!.revision + 1,
        phase: 'editing',
        selected: candidates.find((candidate) => candidate.id === request.candidateId),
      })
      return create(Service.method.selectAuthoringCandidate.output, { session: current })
    })
    rpc(Service.method.cancelAuthoringOperation, () => {
      calls.push('Cancel')
      current = makeSession({
        ...current!,
        revision: current!.revision + 1,
        phase: current!.selected ? 'editing' : 'choosing',
        activeJobId: '',
      })
      return create(Service.method.cancelAuthoringOperation.output, { session: current })
    })
    rpc(Service.method.saveAuthoringSession, async (request) => {
      const captured = current!
      defaults.push(request.makeDefault)
      calls.push('Save')
      if (options.saveGate) await options.saveGate
      if (options.saveConflict) throw connectAppError('AUTHORING_SAVE_CONFLICT', Code.Aborted)
      current = makeSession({
        ...captured,
        revision: captured.revision + 1,
        phase: 'saved',
        saved: { id: 'guideline', kind: captured.kind, name: captured.selected!.name },
      })
      if (options.uncertainSave && publicationUnknown) {
        publicationUnknown = false
        throw new ConnectError('unknown publication delivery', Code.Unavailable)
      }
      return create(Service.method.saveAuthoringSession.output, { session: current })
    })
    rpc(GenerationService.method.getGeneration, () => {
      calls.push('Poll')
      if (!options.hold) finish()
      return create(GenerationService.method.getGeneration.output, {
        job: {
          id: 'job',
          kind: 'authoring',
          status: options.hold ? 'running' : 'done',
          stage: 'write',
        },
      })
    })
  })
  return {
    transport,
    calls,
    starts,
    estimates,
    defaults,
    setSession: (session: Wire) => {
      current = session
    },
    accepted,
    get current() {
      return current
    },
    get latestCalls() {
      return latestCalls
    },
  }
}
function mount(fake: ReturnType<typeof server>, props: Partial<AuthoringEditorProps> = {}) {
  const saved = vi.fn(),
    busy = vi.fn()
  const Component = () => (
    <AuthoringEditor
      ownerId="alice"
      kind="post-guideline"
      onSaved={saved}
      onBusyChange={busy}
      renderPreview={(artifact) => <AuthoringPreview kind="post-guideline" artifact={artifact} />}
      {...props}
    />
  )
  const root = createRootRoute({ component: Component })
  const index = createRoute({ getParentRoute: () => root, path: '/' })
  const router = createRouter({
    routeTree: root.addChildren([index]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const cache = createTestQueryClient()
  return {
    cache,
    ...render(<RouterProvider router={router} />, {
      wrapper: withProviders(fake.transport, cache),
    }),
    saved,
    busy,
  }
}
beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  initializeI18n('ko')
})
async function recommend(user: ReturnType<typeof userEvent.setup>) {
  await waitFor(() => expect(screen.getByRole('button', { name: '8가지 추천받기' })).toBeEnabled())
  await user.click(screen.getByRole('button', { name: '8가지 추천받기' }))
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByText('이번 요청은 최대 5크레딧을 사용해요.')).toBeInTheDocument()
  await user.click(within(dialog).getByRole('button', { name: '이대로 요청하기' }))
}
describe('conversational authoring', () => {
  it('only reads on entry, creates eight together on a confirmed request, and keeps selection separate from save', async () => {
    const fake = server()
    const { saved } = mount(fake)
    const user = userEvent.setup()
    await screen.findByLabelText('어떤 느낌을 원하세요? (선택)')
    expect(fake.calls).toEqual(['GetLatest'])
    expect(screen.queryByRole('combobox')).toBeNull()
    await recommend(user)
    const buttons = await screen.findAllByRole('button', { name: '이 제안 선택하기' })
    expect(buttons).toHaveLength(8)
    expect(fake.calls.filter((call) => call === 'Start')).toHaveLength(1)
    await user.click(buttons[0]!)
    expect(
      await screen.findByText('실제 경험을 바탕으로 1번째 방향으로 써 주세요.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '이 제안 선택하기' })).toBeNull()
    expect(screen.getByRole('button', { name: '다른 제안 보기' })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
    expect(screen.getByRole('heading', { name: '제안 1' })).toHaveFocus()
    expect(fake.calls).not.toContain('Save')
    expect(saved).not.toHaveBeenCalled()
    await user.dblClick(screen.getByRole('button', { name: '이걸로 저장하기' }))
    await waitFor(() => expect(saved).toHaveBeenCalledOnce())
    expect(fake.calls.filter((call) => call === 'Save')).toHaveLength(1)
  })
  it('estimates each chat, updates the visible draft and reply, and does not save automatically', async () => {
    const selected = {
      id: 'existing',
      name: '현재 지침',
      description: '차분한 설명',
      body: '이전 지침의 내용을 보존해요.',
      titleArea: '',
    }
    const fake = server({ initial: makeSession({ phase: 'editing', selected }) })
    mount(fake)
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('어떤 점을 바꿔 볼까요?'), '조금 더 짧게 해 줘')
    await user.dblClick(screen.getByRole('button', { name: '다듬어 주세요' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '이대로 요청하기' }),
    )
    expect(await screen.findByText('짧게 다듬은 지침이에요.')).toBeInTheDocument()
    expect(screen.getByText('조금 더 짧게 다듬었어요.')).toBeInTheDocument()
    expect(fake.starts[0]).toMatchObject({
      prompt: '조금 더 짧게 해 줘',
      model: 'recommended',
      expectedRevision: 1,
    })
    expect(fake.estimates).toEqual([{ sessionId: 'session', model: 'recommended' }])
    expect(fake.starts).toHaveLength(1)
    expect(fake.calls).not.toContain('Save')
  })
  it('reuses the same frozen operation key after an unknown start response', async () => {
    const fake = server({ uncertainStart: true })
    mount(fake)
    const user = userEvent.setup()
    await recommend(user)
    await user.click(await screen.findByRole('button', { name: '다시 확인하고 시도하기' }))
    await screen.findAllByRole('button', { name: '이 제안 선택하기' })
    expect(fake.starts).toHaveLength(2)
    expect(fake.starts[0]!.requestId).toBe(fake.starts[1]!.requestId)
    expect(fake.starts[0]!.expectedRevision).toBe(fake.starts[1]!.expectedRevision)
    expect(fake.accepted.size).toBe(1)
    expect(fake.calls.filter((call) => call === 'Create')).toHaveLength(1)
  })
  it('retains the selected draft and typed message after a publication conflict', async () => {
    const selected = {
      id: 'draft',
      name: '초안',
      description: '현재 제안',
      body: '이 초안은 계속 남아 있어요.',
      titleArea: '',
    }
    const fake = server({
      initial: makeSession({ phase: 'editing', selected }),
      saveConflict: true,
    })
    const { saved } = mount(fake)
    const user = userEvent.setup()
    await user.type(
      await screen.findByLabelText('어떤 점을 바꿔 볼까요?'),
      '입력한 요청도 남아 있어요',
    )
    await user.click(screen.getByRole('button', { name: '이걸로 저장하기' }))
    expect(await screen.findByText(/설정이 다른 곳에서 바뀌었어요/)).toBeInTheDocument()
    expect(screen.getByLabelText('어떤 점을 바꿔 볼까요?')).toHaveValue('입력한 요청도 남아 있어요')
    expect(screen.getByText('이 초안은 계속 남아 있어요.')).toBeInTheDocument()
    expect(saved).not.toHaveBeenCalled()
  })
  it('cancels explicitly without another generation and preserves prior candidates', async () => {
    const fake = server({
      hold: true,
      initial: makeSession({
        phase: 'refining',
        revision: 3,
        activeJobId: 'job',
        selected: { id: 'draft', name: '남은 초안', body: '이전 내용을 보존해요.' },
      }),
    })
    mount(fake)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: '요청 중단하기' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '요청 중단하기' }),
    )
    await waitFor(() => expect(fake.calls).toContain('Cancel'))
    expect(await screen.findByText('이전 내용을 보존해요.')).toBeInTheDocument()
    expect(fake.calls).not.toContain('Start')
  })
  it('never notifies a caller after unmount during a save', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const fake = server({
      initial: makeSession({
        phase: 'editing',
        selected: { id: 'draft', name: '초안', body: '이전 내용' },
      }),
      saveGate: gate,
    })
    const { unmount, saved } = mount(fake)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: '이걸로 저장하기' }))
    unmount()
    await act(async () => release())
    expect(saved).not.toHaveBeenCalled()
  })
  it('uses the supplied default-voice choice visibly and lets the user change it before saving a new copy', async () => {
    const fake = server({
      initial: makeSession({
        kind: ProtoConfigurationKind.WRITING_VOICE,
        phase: 'editing',
        selected: {
          id: 'voice',
          name: '차분한 말투',
          body: '오늘 산책하며 보인 풍경을 차분하게 기록한 가상 예시예요.',
        },
      }),
    })
    const { saved } = mount(fake, { kind: 'writing-voice', initialMakeDefault: true })
    const user = userEvent.setup()
    const checkbox = await screen.findByRole('checkbox', { name: '이 말투를 기본으로 사용하기' })
    expect(checkbox).toBeChecked()
    await user.click(checkbox)
    await user.click(screen.getByRole('button', { name: '새 AI 말투로 저장하기' }))
    await waitFor(() => expect(saved).toHaveBeenCalledOnce())
    expect(fake.defaults).toEqual([false])
    expect(fake.calls).not.toContain('Start')
  })
  it('does not leak an old owner draft or notify the new caller after an owner switch during save', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const fake = server({
      initial: makeSession({
        phase: 'editing',
        selected: { id: 'alice', name: 'Alice의 지침', body: 'Alice의 개인 초안' },
      }),
      saveGate: gate,
    })
    const saved = vi.fn()
    const cache = createTestQueryClient()
    const view = render(
      <AuthoringEditor
        ownerId="alice"
        kind="post-guideline"
        onSaved={saved}
        renderPreview={(artifact) => <AuthoringPreview kind="post-guideline" artifact={artifact} />}
      />,
      { wrapper: withProviders(fake.transport, cache) },
    )
    const user = userEvent.setup()
    await screen.findByText('Alice의 개인 초안')
    await user.click(screen.getByRole('button', { name: '이걸로 저장하기' }))
    await waitFor(() => expect(fake.calls).toContain('Save'))
    fake.setSession(
      makeSession({
        id: 'bob-session',
        phase: 'editing',
        selected: { id: 'bob', name: 'Bob의 지침', body: 'Bob의 새 초안' },
      }),
    )
    view.rerender(
      <AuthoringEditor
        ownerId="bob"
        kind="post-guideline"
        onSaved={saved}
        renderPreview={(artifact) => <AuthoringPreview kind="post-guideline" artifact={artifact} />}
      />,
    )
    await screen.findByText('Bob의 새 초안')
    expect(screen.queryByText('Alice의 개인 초안')).toBeNull()
    await act(async () => {
      release()
      await gate
    })
    expect(saved).not.toHaveBeenCalled()
    expect(screen.getByText('Bob의 새 초안')).toBeInTheDocument()
  })
  it('confirms an uncertain publication from a later read once, while ordinary saved-state reads do not notify', async () => {
    const fake = server({
      initial: makeSession({
        phase: 'editing',
        selected: { id: 'draft', name: '초안', body: '저장할 지침' },
      }),
      uncertainSave: true,
    })
    const { saved, cache } = mount(fake)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: '이걸로 저장하기' }))
    await screen.findByRole('button', { name: '다시 확인하고 시도하기' })
    expect(saved).not.toHaveBeenCalled()
    await cache.invalidateQueries({
      queryKey: authoringSessionQueryKey(
        fake.transport,
        { ownerId: 'alice', kind: 'post-guideline' },
        'session',
      ),
    })
    await waitFor(() => expect(saved).toHaveBeenCalledOnce())
    expect(fake.calls.filter((call) => call === 'Save')).toHaveLength(1)
    cleanup()
    const reopened = mount(fake)
    await screen.findByText('저장했어요. 다음 작업부터 사용할 수 있어요.')
    expect(reopened.saved).not.toHaveBeenCalled()
  })
})
