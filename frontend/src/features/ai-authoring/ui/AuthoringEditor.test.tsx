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
  ProtoAuthoringDraftState,
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
    candidateCount: number
  }> = []
  const estimates: Array<{ sessionId: string; model: string }> = []
  const defaults: boolean[] = []
  const accepted = new Map<string, Wire>()
  let requestedCount = 8
  let uncertain = true
  let publicationUnknown = true
  const candidates = Array.from({ length: 16 }, (_, i) => ({
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
        candidateCount: requestedCount,
        candidates: candidates.slice(0, requestedCount),
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
        candidateCount: request.candidateCount,
      })
      if (options.uncertainStart && uncertain) {
        uncertain = false
        throw new ConnectError('unknown delivery', Code.Unavailable)
      }
      if (!accepted.has(request.requestId)) {
        if (request.mode === 1) requestedCount = request.candidateCount || 8
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
    rpc(Service.method.patchAuthoringDraft, (request) => {
      calls.push('Patch')
      const source = request.workingSource!
      const valid = source.body.trim() !== '' && source.body !== '<invalid'
      current = makeSession({
        ...current!,
        revision: current!.revision + 1,
        phase: 'editing',
        workingSource: { ...source, id: current!.selected?.id ?? 'manual' },
        draftState: valid
          ? ProtoAuthoringDraftState.VALID
          : source.body.trim()
            ? ProtoAuthoringDraftState.INVALID
            : ProtoAuthoringDraftState.INCOMPLETE,
        hasUnpublishedChanges: true,
        selected: valid ? { ...source, id: current!.selected?.id ?? 'manual' } : current!.selected,
      })
      return create(Service.method.patchAuthoringDraft.output, { session: current })
    })
    rpc(Service.method.resetAuthoringChat, () => {
      calls.push('ResetChat')
      current = makeSession({
        ...current!,
        revision: current!.revision + 1,
        phase: 'editing',
        turns: [],
        pendingRequest: '',
      })
      return create(Service.method.resetAuthoringChat.output, { session: current })
    })
    rpc(Service.method.resetAuthoringBaseline, () => {
      calls.push('ResetBaseline')
      current = makeSession({
        ...current!,
        revision: current!.revision + 1,
        phase: 'editing',
        workingSource: current!.savedBaseline,
        selected: current!.savedBaseline,
        draftState: ProtoAuthoringDraftState.VALID,
        hasUnpublishedChanges: false,
        turns: [],
      })
      return create(Service.method.resetAuthoringBaseline.output, { session: current })
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
const initialViewport = window.innerWidth

afterEach(() => {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: initialViewport })
  delete document.documentElement.dataset.theme
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
    await user.click(await screen.findByRole('button', { name: /AI로 편집하기$/ }))
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
    await user.click(await screen.findByRole('button', { name: /AI로 편집하기$/ }))
    await user.type(
      await screen.findByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' }),
      '입력한 요청도 남아 있어요',
    )
    await user.click(screen.getByRole('button', { name: '결과 확인으로 돌아가기' }))
    await publication(user)
    await user.click(screen.getByRole('button', { name: '이걸로 저장하기' }))
    expect(await screen.findByText(/설정이 다른 곳에서 바뀌었어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '돌아가기' }))
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
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
    await screen.findByText(/새로 저장했어요/)
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
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
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
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
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
    await screen.findByRole('button', { name: /작문 지침 확인하기$/ })
    expect(fake.calls).toEqual(['GetLatest'])
    await user.click(screen.getByRole('button', { name: /작문 지침 확인하기$/ }))
    expect(await screen.findByText('현재 지침을 초안으로 불러왔어요.')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).toBeNull()
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
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
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
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
    await user.click(await screen.findByRole('button', { name: /AI로 편집하기$/ }))
    expect(screen.getByText(/이 대화에서 스무 번을 다듬었어요/)).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('button', { name: '다듬어 주세요' })).toBeNull()
    await user.click(screen.getByRole('button', { name: '결과 확인으로 돌아가기' }))
    await publication(user)
    expect(fake.calls).not.toContain('Estimate')
    expect(fake.calls).not.toContain('Start')
  })
})

describe('one named AI and direct working draft', () => {
  it.each(['day', 'night'])(
    'keeps invalid direct source, changes methods without spending, and separates chat reset from discard (%s)',
    async (theme) => {
      document.documentElement.dataset.theme = theme
      const baseline = {
        id: 'draft',
        name: '동네 산책 지침',
        description: '담백한 글',
        body: '저장한 지침 내용',
        titleArea: '',
      }
      const fake = server({
        initial: makeSession({
          phase: 'editing',
          targetId: 'owned-guide',
          targetVersion: 'captured',
          savedAvailable: true,
          savedBaseline: baseline,
          workingSource: baseline,
          selected: baseline,
          turns: [{ id: 'turn', request: '이전 요청', reply: '이전 답변', status: 'done' }],
        }),
      })
      const view = mount(fake, { targetId: 'owned-guide' })
      const user = userEvent.setup()
      await user.click(
        await screen.findByRole('button', { name: '“동네 산책 지침” 직접 편집하기' }),
      )
      expect(screen.getByRole('heading', { name: '직접 내용을 편집해 주세요' })).toBeInTheDocument()
      fireEvent.change(screen.getByRole('textbox', { name: '내용' }), {
        target: { value: '<invalid' },
      })
      await user.click(screen.getByRole('button', { name: '“동네 산책 지침” AI로 편집하기' }))
      await screen.findByRole('heading', { name: '어떤 점을 바꿔 볼까요?' })
      expect(fake.current?.workingSource?.body).toBe('<invalid')
      expect(fake.current?.selected?.body).toBe(baseline.body)
      expect(screen.getByText(/미완성 또는 올바르지 않은 입력을 보관했어요/)).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: '새 대화 시작하기' }))
      await waitFor(() => expect(fake.calls).toContain('ResetChat'))
      expect(fake.current?.workingSource?.body).toBe('<invalid')
      expect(fake.current?.turns).toHaveLength(0)
      expect(fake.calls).not.toContain('ResetBaseline')
      // Reopen under another viewport: durable source survives and no request is reissued.
      view.unmount()
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1440 })
      mount(fake, { targetId: 'owned-guide' })
      await user.click(
        await screen.findByRole('button', { name: '“동네 산책 지침” 직접 편집하기' }),
      )
      expect(screen.getByRole('textbox', { name: '내용' })).toHaveValue('<invalid')
      await user.click(screen.getByRole('button', { name: '저장한 “동네 산책 지침”으로 되돌리기' }))
      const dialog = await screen.findByRole('dialog')
      expect(within(dialog).getByText(/저장하지 않은 편집 내용을 버리고/)).toBeInTheDocument()
      expect(fake.calls).not.toContain('ResetBaseline')
      await user.click(within(dialog).getByRole('button', { name: '변경사항 버리기' }))
      await waitFor(() => expect(fake.current?.workingSource?.body).toBe(baseline.body))
      expect(fake.calls.filter((call) => ['Start', 'Estimate', 'Save'].includes(call))).toEqual([])
      delete document.documentElement.dataset.theme
    },
  )
})

it.each([2, 4, 16] as const)(
  'quotes and explicitly requests exactly %d unsaved candidates with truthful count copy',
  async (count) => {
    const fake = server()
    mount(fake, { candidateCount: count })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: `${count}가지 추천받기` }))
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByRole('heading', { name: `${count}가지 제안을 준비할까요?` }),
    ).toBeInTheDocument()
    expect(fake.starts).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: '이대로 요청하기' }))
    await screen.findByRole('button', { name: `제안 ${count}` })
    expect(fake.current?.candidates).toHaveLength(count)
    expect(fake.starts).toHaveLength(1)
    expect(fake.starts[0].candidateCount).toBe(count)
    expect(fake.calls).not.toContain('Save')
  },
)
it('opens an untitled saved guideline with its readable content identity and confirms its named update', async () => {
  const baseline = {
    id: 'draft',
    name: '',
    body: '짧고 읽기 쉬운 문장으로 써 주세요.',
    description: '',
    titleArea: '',
  }
  const fake = server({
    initial: makeSession({
      phase: 'editing',
      targetId: 'owned-guide',
      selected: baseline,
      workingSource: baseline,
      savedBaseline: baseline,
      savedAvailable: true,
    }),
  })
  mount(fake, { targetId: 'owned-guide' })
  const user = userEvent.setup()
  expect(
    await screen.findByRole('button', {
      name: '“짧고 읽기 쉬운 문장으로 써 주세요.” 직접 편집하기',
    }),
  ).toBeInTheDocument()
  await publication(user)
  await user.click(screen.getByRole('button', { name: '변경사항 적용하기' }))
  expect(
    await screen.findByText(
      '“짧고 읽기 쉬운 문장으로 써 주세요.” 작문 지침의 변경사항을 저장했어요.',
    ),
  ).toBeInTheDocument()
  expect(fake.starts).toHaveLength(0)
})

it('reviews a personal voice without a synthetic example and offers AI preparation without starting work', async () => {
  const baseline = {
    id: 'personal',
    name: '내 말투',
    description: '저장한 문장 특징',
    body: '',
    titleArea: '',
  }
  const fake = server({
    initial: makeSession({
      kind: ProtoConfigurationKind.WRITING_VOICE,
      phase: 'editing',
      targetId: 'personal',
      selected: baseline,
      workingSource: baseline,
      savedBaseline: baseline,
      savedAvailable: true,
      draftState: ProtoAuthoringDraftState.INCOMPLETE,
    }),
  })
  mount(fake, { kind: 'writing-voice', targetId: 'personal' })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '말투 예문 만들어 보기' }))
  expect(screen.getByRole('heading', { name: '어떤 점을 바꿔 볼까요?' })).toBeInTheDocument()
  expect(screen.getByText('저장한 문장 특징')).toBeInTheDocument()
  expect(fake.starts).toHaveLength(0)
  expect(fake.calls).not.toContain('Save')
})

it('recovers the explicitly chosen new creation without loading the latest conversation or starting paid work', async () => {
  const fake = server({
    initial: makeSession({
      id: 'chosen-creation',
      phase: 'editing',
      selected: { id: 'draft', name: '이어 쓸 지침', body: '이전에 준비한 지침이에요.' },
    }),
  })
  mount(fake, { sessionId: 'chosen-creation' })
  expect(await screen.findByText('이전에 준비한 지침이에요.')).toBeInTheDocument()
  expect(fake.calls).toContain('Get')
  expect(fake.calls).not.toContain('GetLatest')
  expect(fake.starts).toHaveLength(0)
  expect(fake.calls).not.toContain('Create')
  expect(fake.calls).not.toContain('Save')
})

it('keeps an eight-entry preview while preparing two new choices and opens the completed new choices without publishing', async () => {
  const candidates = Array.from({ length: 8 }, (_, i) => ({
    id: `old-${i}`,
    name: `이전 ${i + 1}`,
    description: '이전 방향',
    body: `이전 내용 ${i + 1}`,
    titleArea: '',
  }))
  const fake = server({
    initial: makeSession({
      phase: 'choosing',
      candidateCount: 8,
      candidates,
      selected: candidates[0],
    }),
  })
  mount(fake, { candidateCount: 2 })
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: '다른 2가지 추천받기' }))
  const dialog = await screen.findByRole('dialog')
  await user.click(within(dialog).getByRole('button', { name: '이대로 요청하기' }))
  expect(await screen.findByRole('button', { name: '제안 2' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '제안 3' })).toBeNull()
  expect(fake.current?.candidates).toHaveLength(2)
  expect(fake.current?.selected?.body).toBe('이전 내용 1')
  expect(fake.calls).not.toContain('Save')
})

it('restores a persisted target conflict as a conflict instead of presenting an ordinary uncertain save', async () => {
  const baseline = {
    id: 'draft',
    name: '저장 지침',
    description: '',
    body: '유지할 지침',
    titleArea: '',
  }
  const fake = server({
    initial: makeSession({
      phase: 'saving',
      targetId: 'owned-guide',
      failureReason: 'AUTHORING_SAVE_CONFLICT',
      savedAvailable: true,
      savedBaseline: baseline,
      workingSource: baseline,
      selected: baseline,
    }),
  })
  mount(fake, { targetId: 'owned-guide' })
  expect(await screen.findByRole('alert')).toHaveTextContent('설정이 다른 곳에서 바뀌었어요')
  expect(screen.getByRole('button', { name: '저장 결과 다시 확인하기' })).toBeInTheDocument()
  expect(fake.calls).not.toContain('Save')
  expect(fake.starts).toHaveLength(0)
})

it('recovers unfinished new manual work with no valid preview and can switch to AI without losing the source or spending', async () => {
  const source = { id: 'manual', name: '준비 중인 구성', description: '', body: '', titleArea: '' }
  const fake = server({
    initial: makeSession({
      id: 'unfinished',
      kind: ProtoConfigurationKind.POST_TEMPLATE,
      phase: 'editing',
      workingSource: source,
      hasUnpublishedChanges: true,
      draftState: ProtoAuthoringDraftState.INCOMPLETE,
    }),
  })
  mount(fake, { kind: 'post-template', sessionId: 'unfinished' })
  const user = userEvent.setup()
  expect(await screen.findByRole('textbox', { name: '이름' })).toHaveValue(source.name)
  expect(screen.getByRole('textbox', { name: '내용' })).toHaveValue('')
  await user.click(screen.getByRole('button', { name: '“준비 중인 구성” AI로 편집하기' }))
  expect(screen.getByRole('textbox', { name: '어떤 점을 바꿔 볼까요?' })).toBeInTheDocument()
  expect(fake.current?.workingSource?.name).toBe(source.name)
  expect(fake.current?.selected).toBeUndefined()
  expect(fake.starts).toHaveLength(0)
  expect(fake.calls).not.toContain('Save')
})
