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
import { cleanup, render, screen, within, waitFor, act, fireEvent } from '@testing-library/react'
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
      current =
        current ??
        makeSession({
          kind: request.kind,
          targetId: request.targetId,
          ...(request.targetId
            ? {
                phase: 'editing',
                targetVersion: 'captured-version',
                selected: {
                  id: request.targetId,
                  name: '저장한 지침',
                  body: '현재 지침을 초안으로 불러왔어요.',
                },
              }
            : {}),
        })
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
async function publication(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '저장할 내용 확인하기' }))
  await screen.findByRole('heading', { name: '저장할 내용을 확인해 주세요' })
}
describe('conversational authoring', () => {
  it('only reads on entry, creates eight together on a confirmed request, and keeps selection separate from save', async () => {
    const fake = server()
    const { saved } = mount(fake)
    const user = userEvent.setup()
    await screen.findByRole('textbox', { name: '글을 쓸 때 어떤 점을 지키면 좋을까요?' })
    expect(fake.calls).toEqual(['GetLatest'])
    expect(screen.queryByRole('combobox')).toBeNull()
    await recommend(user)
    const buttons = await screen.findAllByRole('button', { name: /^제안 [1-8]$/ })
    expect(buttons).toHaveLength(8)
    expect(fake.calls.filter((call) => call === 'Start')).toHaveLength(1)
    await user.click(buttons[0]!)
    expect(
      await screen.findByText('실제 경험을 바탕으로 1번째 방향으로 써 주세요.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^제안 [1-8]$/ })).toBeNull()
    expect(screen.getByRole('button', { name: '돌아가기' })).toBeEnabled()
    expect(screen.getByRole('heading', { name: '선택한 결과를 확인해 주세요' })).toHaveFocus()
    expect(fake.calls).not.toContain('Save')
    expect(saved).not.toHaveBeenCalled()
    await publication(user)
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
    await user.click(await screen.findByRole('button', { name: '조금 다듬기' }))
    await user.type(
      await screen.findByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' }),
      '조금 더 짧게 해 줘',
    )
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
    await screen.findAllByRole('button', { name: /^제안 [1-8]$/ })
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
    await user.click(await screen.findByRole('button', { name: '조금 다듬기' }))
    await user.type(
      await screen.findByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' }),
      '입력한 요청도 남아 있어요',
    )
    await user.click(screen.getByRole('button', { name: '결과 확인으로 돌아가기' }))
    await publication(user)
    await user.click(screen.getByRole('button', { name: '이걸로 저장하기' }))
    expect(await screen.findByText(/설정이 다른 곳에서 바뀌었어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '돌아가기' }))
    await user.click(screen.getByRole('button', { name: '조금 다듬기' }))
    expect(screen.getByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' })).toHaveValue(
      '입력한 요청도 남아 있어요',
    )
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
    await publication(user)
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
    await publication(user)
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
    await publication(user)
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
    await publication(user)
    await user.click(await screen.findByRole('button', { name: '이걸로 저장하기' }))
    await screen.findByRole('button', { name: '저장 결과 다시 확인하기' })
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
  it('preserves the purpose when going back from eight suggestions without another request', async () => {
    const fake = server()
    mount(fake)
    const user = userEvent.setup()
    const purpose = await screen.findByRole('textbox', {
      name: '글을 쓸 때 어떤 점을 지키면 좋을까요?',
    })
    await user.type(purpose, '경험을 편하게 설명하고 싶어요')
    await recommend(user)
    await screen.findAllByRole('button', { name: /^제안 [1-8]$/ })
    await user.click(screen.getByRole('button', { name: '돌아가기' }))
    expect(
      screen.getByRole('textbox', { name: '글을 쓸 때 어떤 점을 지키면 좋을까요?' }),
    ).toHaveValue('경험을 편하게 설명하고 싶어요')
    expect(fake.starts).toHaveLength(1)
    expect(fake.calls).not.toContain('Select')
    expect(fake.calls).not.toContain('Save')
  })
  it('explains chat input on an attempt and retains it across review and publication Back', async () => {
    const fake = server({
      initial: makeSession({
        phase: 'editing',
        selected: { id: 'draft', name: '현재 결과', body: '저장 가능한 지침이에요.' },
      }),
    })
    mount(fake)
    const user = userEvent.setup()
    await screen.findByRole('button', { name: '저장할 내용 확인하기' })
    expect(screen.queryByRole('textbox')).toBeNull()
    await user.click(screen.getByRole('button', { name: '조금 다듬기' }))
    const send = screen.getByRole('button', { name: '다듬어 주세요' })
    expect(send).toBeEnabled()
    await user.click(send)
    expect(screen.getByRole('alert')).toHaveTextContent('바꾸고 싶은 점을 적어 주세요')
    const field = screen.getByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' })
    fireEvent.change(field, { target: { value: '가'.repeat(2001) } })
    await user.click(send)
    expect(screen.getByRole('alert')).toHaveTextContent('2000자 안으로 적어 주세요')
    expect(field).toHaveValue('가'.repeat(2001))
    fireEvent.change(field, { target: { value: '아직 보내지 않은 요청이에요' } })
    await user.click(screen.getByRole('button', { name: '결과 확인으로 돌아가기' }))
    await publication(user)
    await user.click(screen.getByRole('button', { name: '돌아가기' }))
    await user.click(screen.getByRole('button', { name: '조금 다듬기' }))
    expect(screen.getByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' })).toHaveValue(
      '아직 보내지 않은 요청이에요',
    )
    expect(fake.calls).not.toContain('Estimate')
    expect(fake.calls).not.toContain('Start')
    expect(fake.calls).not.toContain('Save')
  })
  it('browses and reuses the current choice without another selection mutation', async () => {
    const fake = server()
    mount(fake)
    const user = userEvent.setup()
    await recommend(user)
    await user.click(await screen.findByRole('button', { name: '제안 1' }))
    await screen.findByRole('button', { name: '저장할 내용 확인하기' })
    await user.click(screen.getByRole('button', { name: '돌아가기' }))
    const current = screen.getByRole('button', { name: '제안 1' })
    expect(current).toHaveAttribute('aria-pressed', 'true')
    await user.click(current)
    await screen.findByRole('button', { name: '저장할 내용 확인하기' })
    expect(fake.calls.filter((call) => call === 'Select')).toHaveLength(1)
    expect(fake.starts).toHaveLength(1)
  })
  it('loads an existing target only on request and never recommends to start editing it', async () => {
    const fake = server()
    mount(fake, { targetId: 'owned-guide' })
    const user = userEvent.setup()
    await screen.findByRole('button', { name: '수정 초안 불러오기' })
    expect(fake.calls).toEqual(['GetLatest'])
    await user.click(screen.getByRole('button', { name: '수정 초안 불러오기' }))
    expect(await screen.findByText('현재 지침을 초안으로 불러왔어요.')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).toBeNull()
    await user.click(screen.getByRole('button', { name: '조금 다듬기' }))
    expect(screen.getByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' })).toBeEnabled()
    expect(fake.calls.filter((call) => call === 'Create')).toHaveLength(1)
    expect(fake.calls).not.toContain('Estimate')
    expect(fake.calls).not.toContain('Start')
  })
  it('offers explicit recovery for an interrupted publication even while its durable phase is saving', async () => {
    const fake = server({
      initial: makeSession({
        phase: 'saving',
        selected: { id: 'draft', name: '저장 중인 지침', body: '확인할 지침이에요.' },
        activeJobId: '',
      }),
    })
    const { saved } = mount(fake)
    const user = userEvent.setup()
    const recover = await screen.findByRole('button', { name: '저장 결과 다시 확인하기' })
    expect(recover).toBeEnabled()
    await user.click(recover)
    await waitFor(() => expect(saved).toHaveBeenCalledOnce())
    expect(fake.calls.filter((call) => call === 'Save')).toHaveLength(1)
    expect(fake.calls).not.toContain('Start')
  })
  it('keeps hidden actors alive without moving focus and exposes an actor-guarded parent Back port', async () => {
    const fake = server({
      initial: makeSession({
        phase: 'editing',
        selected: { id: 'draft', name: '숨긴 초안', body: '유지할 지침이에요.' },
      }),
    })
    const navigation = vi.fn()
    const cache = createTestQueryClient()
    const props = {
      ownerId: 'alice',
      kind: 'post-guideline' as const,
      onNavigationChange: navigation,
    }
    const view = render(<AuthoringEditor {...props} active={false} />, {
      wrapper: withProviders(fake.transport, cache),
    })
    const user = userEvent.setup()
    await screen.findByText('유지할 지침이에요.')
    expect(screen.getByRole('heading', { name: '선택한 결과를 확인해 주세요' })).not.toHaveFocus()
    expect(screen.queryByRole('button', { name: '돌아가기' })).toBeNull()
    view.rerender(<AuthoringEditor {...props} active />)
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: '선택한 결과를 확인해 주세요' })).toHaveFocus(),
    )
    await user.click(screen.getByRole('button', { name: '조금 다듬기' }))
    const port = navigation.mock.calls.at(-1)![0]!
    expect(port.canGoBack).toBe(true)
    await act(async () => port.goBack())
    expect(screen.getByRole('button', { name: '저장할 내용 확인하기' })).toBeInTheDocument()
    expect(fake.latestCalls).toBe(1)
    expect(fake.calls).not.toContain('Start')
  })
  it('offers review when the bounded conversation is complete without showing an unusable send form', async () => {
    const fake = server({
      initial: makeSession({
        phase: 'editing',
        selected: { id: 'draft', name: '완성된 지침', body: '이 결과를 사용할 수 있어요.' },
        turns: Array.from({ length: 20 }, (_, index) => ({
          id: 'turn-' + index,
          request: '수정 요청',
          reply: '다듬었어요.',
          jobId: 'job-' + index,
          status: 'done',
        })),
      }),
    })
    mount(fake)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: '조금 다듬기' }))
    expect(screen.getByText(/이 대화에서 스무 번을 다듬었어요/)).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('button', { name: '다듬어 주세요' })).toBeNull()
    await user.click(screen.getByRole('button', { name: '결과 확인으로 돌아가기' }))
    await publication(user)
    expect(fake.calls).not.toContain('Estimate')
    expect(fake.calls).not.toContain('Start')
  })
})
