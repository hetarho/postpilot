import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { RequestInspectionTarget } from '@/entities/request-inspection'
import {
  AuthService,
  FragmentAuthorship,
  InspectionRole,
  InspectionStatus,
  ProtoAuthoringMode,
  ProtoConfigurationKind,
  RequestInspectionSchema,
  WritingInspectionService,
  type RequestInspection,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { InspectWritingRequestAction } from './InspectWritingRequestAction'

const post: RequestInspectionTarget = {
  ownerId: 'alice',
  kind: 'post',
  postSlug: 'owned-post',
  sourceRevision: 'source-1',
  resultRevision: 'result-1',
}

const authoring: Extract<RequestInspectionTarget, { kind: 'authoring' }> = {
  ownerId: 'alice',
  kind: 'authoring',
  sessionId: 'owned-session',
  authoringKind: 'post-template',
  revision: 3,
  mode: 'refine',
  prompt: '주장을 줄여 주세요.',
  candidateCount: 2,
  model: { providerId: 'provider', modelId: 'model' },
  candidateId: 'selected',
  operationId: 'operation',
}

function inspection(status = InspectionStatus.CURRENT, callId?: string): RequestInspection {
  return create(RequestInspectionSchema, {
    version: 1,
    status,
    stage: 'post-writing',
    mode: 'post-writing',
    promptVersion: 'prompt-v1',
    schemaVersion: 'schema-v2',
    fragments: [
      {
        id: 'service-rule',
        role: InspectionRole.SYSTEM,
        authorship: FragmentAuthorship.CODE,
        materialRole: 'output rules',
        text: '서비스가 정의한 사실만 써 주세요.',
        sourceFiles: ['internal/writing/prompt.go'],
        activation: 'explicit writing',
      },
      {
        id: 'owner-facts',
        role: InspectionRole.USER,
        authorship: FragmentAuthorship.ACCOUNT,
        materialRole: 'owner facts',
        text: '내가 쓴 메모',
        sourceRefs: ['memo'],
        activation: 'selected owner material',
      },
    ],
    selectedRuleIds: ['facts-only', 'output-contract'],
    omissions: [
      {
        id: 'inactive-guideline',
        reason: 'No guideline was selected.',
        activation: 'guideline absent',
        sourceFiles: ['internal/writing/prompt.go'],
      },
    ],
    nativeFields: [
      {
        id: 'grammar',
        authorship: FragmentAuthorship.CODE,
        materialRole: 'output shape',
        text: '<write>본문</write>',
        sourceFiles: ['internal/writing/schema.go'],
      },
    ],
    output: { name: 'canonical-post', version: 'output-v2', schema: '{"required":["body"]}' },
    conditions: {
      model: { providerId: 'provider', modelId: 'model' },
      maxCompletionTokens: 1024n,
      reasoningEffort: 'low',
      structuredOutput: true,
    },
    measures: {
      characters: 123n,
      utf8Bytes: 321n,
      referenceTokenEstimate: 81n,
      ...(status === InspectionStatus.CAPTURED
        ? { providerPromptTokens: 90n, providerCompletionTokens: 20n }
        : {}),
    },
    ...(status === InspectionStatus.CAPTURED
      ? { callId: callId ?? 'issued-call', issuedAt: { seconds: 1000n, nanos: 0 } }
      : {}),
    composer: 'writing.compose',
    parser: 'writing.parse',
    consumer: 'post.content',
    sourceFiles: ['internal/writing/prompt.go'],
  })
}

function harness(
  options: {
    read?: (
      status: InspectionStatus,
      stage: string,
    ) => RequestInspection | RequestInspection[] | Promise<RequestInspection>
  } = {},
) {
  const calls: Array<{ name: string; input: unknown }> = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(WritingInspectionService.method.getPostRequestInspection, async (request) => {
      calls.push({ name: 'GetPostRequestInspection', input: request })
      const value = await (options.read?.(request.status, request.stage) ??
        inspection(request.status))
      return create(
        WritingInspectionService.method.getPostRequestInspection.output,
        Array.isArray(value) ? { inspections: value } : { inspection: value },
      )
    })
    rpc(WritingInspectionService.method.getWritingTestRequestInspection, async (request) => {
      calls.push({ name: 'GetWritingTestRequestInspection', input: request })
      const value = await (options.read?.(request.status, request.stage) ??
        inspection(request.status))
      return create(
        WritingInspectionService.method.getWritingTestRequestInspection.output,
        Array.isArray(value) ? { inspections: value } : { inspection: value },
      )
    })
    rpc(WritingInspectionService.method.getAuthoringRequestInspection, async (request) => {
      calls.push({ name: 'GetAuthoringRequestInspection', input: request })
      const value = await (options.read?.(request.status, request.stage) ??
        inspection(request.status))
      return create(WritingInspectionService.method.getAuthoringRequestInspection.output, {
        inspection: Array.isArray(value) ? value[0] : value,
      })
    })
  })
  const cache = createTestQueryClient()
  const ownerKey = createConnectQueryKey({
    schema: AuthService.method.getMe,
    input: {},
    transport,
    cardinality: 'finite',
  })
  const setOwner = (id: string) =>
    cache.setQueryData(ownerKey, create(AuthService.method.getMe.output, { user: { id } }))
  setOwner('alice')
  return { calls, cache, setOwner, wrapper: withProviders(transport, cache) }
}

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.removeAttribute('data-theme')
})

describe('optional technical request inspection', () => {
  it('opens named technical evidence with ordered wire roles and independent ownership, and keeps grammar inside the view', async () => {
    const user = userEvent.setup()
    const backend = harness()
    render(<InspectWritingRequestAction target={post} stages={['write', 'observe']} />, {
      wrapper: backend.wrapper,
    })
    expect(backend.calls).toEqual([])
    expect(screen.queryByText('내가 쓴 메모')).not.toBeInTheDocument()
    expect(screen.queryByText('<write>본문</write>')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    const sheet = screen.getByRole('dialog', { name: '요청 기술 보기' })
    expect(await within(sheet).findByText('내가 쓴 메모')).toBeInTheDocument()
    expect(within(sheet).getByText('글 작성 · post-writing')).toBeInTheDocument()
    expect(within(sheet).getByRole('heading', { name: '요청 조각 1 · System' })).toBeInTheDocument()
    expect(within(sheet).getByRole('heading', { name: '요청 조각 2 · User' })).toBeInTheDocument()
    expect(within(sheet).getAllByText('서비스 코드')).toHaveLength(2)
    expect(within(sheet).getByText('내 계정 자료')).toBeInTheDocument()
    expect(within(sheet).getByText('No guideline was selected.')).toBeInTheDocument()
    expect(within(sheet).getByText('facts-only')).toBeInTheDocument()
    expect(
      within(sheet).getByText('현재 설정과 자료를 보여 줍니다. 실제 전송한 요청이 아닙니다.'),
    ).toHaveClass('text-base')
    expect(within(sheet).queryByText('{"required":["body"]}')).not.toBeInTheDocument()
    await user.click(within(sheet).getByRole('button', { name: '출력 스키마·원문 문법' }))
    expect(within(sheet).getByText('{"required":["body"]}')).toHaveClass(
      'text-base',
      'whitespace-pre-wrap',
    )
    expect(backend.calls).toHaveLength(1)
  })

  it('reopens and copies only read projections without additional calls, holds or canonical writing mutations', async () => {
    const user = userEvent.setup()
    const write = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    const backend = harness()
    render(<InspectWritingRequestAction target={post} />, { wrapper: backend.wrapper })
    const opener = screen.getByRole('button', { name: '요청 기술 보기' })
    for (let iteration = 0; iteration < 2; iteration += 1) {
      await user.click(opener)
      const sheet = screen.getByRole('dialog')
      await within(sheet).findByText('내가 쓴 메모')
      await user.click(within(sheet).getByRole('button', { name: '기술 요청 복사' }))
      await user.click(within(sheet).getByRole('button', { name: '기술 요청 복사' }))
      expect(await within(sheet).findByText('기술 요청을 복사했어요.')).toBeInTheDocument()
      expect(backend.calls).toHaveLength(iteration + 1)
      await user.keyboard('{Escape}')
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(opener).toHaveFocus()
    }
    expect(write).toHaveBeenCalledTimes(4)
    expect(write.mock.calls[0]?.[0]).toContain('State: current')
    expect(write.mock.calls[0]?.[0]).toContain('Material owner: account')
    expect(write.mock.calls[0]?.[0]).not.toContain('$typeName')
    expect(backend.calls.map((call) => call.name)).toEqual([
      'GetPostRequestInspection',
      'GetPostRequestInspection',
    ])
    await waitFor(() =>
      expect(
        backend.cache.getQueryCache().findAll({ queryKey: ['request-inspection'] }),
      ).toHaveLength(0),
    )
  })

  it('distinguishes prepared previews and each actual captured call, with unknown usage separate from estimates', async () => {
    const user = userEvent.setup()
    const backend = harness({
      read: (status) =>
        status === InspectionStatus.CAPTURED
          ? [inspection(status, 'call-first'), inspection(status, 'call-second')]
          : inspection(status),
    })
    render(<InspectWritingRequestAction target={post} />, { wrapper: backend.wrapper })
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    const sheet = screen.getByRole('dialog')
    await within(sheet).findByText('내가 쓴 메모')
    await user.click(within(sheet).getByRole('combobox', { name: '요청 상태 현재 설정' }))
    await user.click(screen.getByRole('option', { name: '준비 미리보기' }))
    expect(
      await within(sheet).findByText(
        '현재 조건에서 준비한 요청입니다. 실행하거나 전송한 기록이 아닙니다.',
      ),
    ).toBeInTheDocument()
    const inputRow = within(sheet).getByText('제공자 실제 입력 토큰').parentElement!
    expect(inputRow).toHaveTextContent('알 수 없음')
    const estimateRow = within(sheet).getByText('참고 토큰 추정치').parentElement!
    expect(estimateRow).toHaveTextContent('81')
    expect(within(sheet).queryByText('전송 식별자')).not.toBeInTheDocument()
    await user.click(within(sheet).getByRole('combobox', { name: '요청 상태 준비 미리보기' }))
    await user.click(screen.getByRole('option', { name: '실제 전송 기록' }))
    expect(await within(sheet).findByText('call-first')).toBeInTheDocument()
    expect(within(sheet).getByText('call-second')).toBeInTheDocument()
    expect(within(sheet).getAllByText('전송 식별자')).toHaveLength(2)
    expect(within(sheet).getAllByText('제공자 실제 입력 토큰')[0]?.parentElement).toHaveTextContent(
      '90',
    )
    expect(backend.calls).toHaveLength(3)
  })

  it('shows an unsaved-material reason with zero reads and clears an open view when material changes', async () => {
    const user = userEvent.setup()
    const backend = harness()
    const view = render(
      <InspectWritingRequestAction
        target={post}
        blockedReason="수정한 자료를 저장한 뒤 확인할 수 있습니다."
      />,
      { wrapper: backend.wrapper },
    )
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    expect(screen.getByRole('dialog')).toHaveTextContent(
      '수정한 자료를 저장한 뒤 확인할 수 있습니다.',
    )
    expect(backend.calls).toEqual([])
    view.rerender(<InspectWritingRequestAction target={post} contextKey="new-local-material" />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(backend.calls).toEqual([])
  })

  it.each([
    ['source revision', { ...post, sourceRevision: 'source-2' }],
    ['result revision', { ...post, resultRevision: 'result-2' }],
    ['resource', { ...post, postSlug: 'other-post' }],
    ['owner', { ...post, ownerId: 'bob' }],
  ] as const)(
    'dismisses on %s changes and stays closed when the prior identity returns',
    async (_name, changed) => {
      const user = userEvent.setup()
      const backend = harness()
      const rendered = render(<InspectWritingRequestAction target={post} />, {
        wrapper: backend.wrapper,
      })
      await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
      await screen.findByText('내가 쓴 메모')
      rendered.rerender(<InspectWritingRequestAction target={changed} />)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(screen.queryByText('내가 쓴 메모')).not.toBeInTheDocument()
      rendered.rerender(<InspectWritingRequestAction target={post} />)
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(backend.calls).toHaveLength(1)
      await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
      await screen.findByText('내가 쓴 메모')
      expect(backend.calls).toHaveLength(2)
    },
  )

  it('dismisses on host context changes and does not reopen after undoing those edits', async () => {
    const user = userEvent.setup()
    const backend = harness()
    const rendered = render(<InspectWritingRequestAction target={post} contextKey="material-A" />, {
      wrapper: backend.wrapper,
    })
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    await screen.findByText('내가 쓴 메모')
    const initialKey = backend.cache
      .getQueryCache()
      .findAll({ queryKey: ['request-inspection'] })[0]!.queryKey
    expect(JSON.stringify(initialKey)).toContain('material-A')
    rendered.rerender(<InspectWritingRequestAction target={post} contextKey="material-B" />)
    rendered.rerender(<InspectWritingRequestAction target={post} contextKey="material-A" />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(backend.calls).toHaveLength(1)
    rendered.rerender(<InspectWritingRequestAction target={post} contextKey="material-B" />)
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    await screen.findByText('내가 쓴 메모')
    const nextKey = backend.cache.getQueryCache().findAll({ queryKey: ['request-inspection'] })[0]!
      .queryKey
    expect(JSON.stringify(nextKey)).toContain('material-B')
    expect(nextKey).not.toEqual(initialKey)
  })

  it('reads the exact prospective setting request and invalidates it when its inputs change', async () => {
    const user = userEvent.setup()
    const backend = harness({
      read: (status) => {
        const value = inspection(status)
        value.stage = 'setting-authoring'
        value.mode = 'post-template/refine'
        return value
      },
    })
    const rendered = render(
      <InspectWritingRequestAction
        target={authoring}
        stages={['setting-authoring']}
        defaultStatus="prepared"
      />,
      { wrapper: backend.wrapper },
    )
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    await screen.findByText('내가 쓴 메모')
    expect(backend.calls[0]).toMatchObject({
      name: 'GetAuthoringRequestInspection',
      input: {
        sessionId: 'owned-session',
        kind: ProtoConfigurationKind.POST_TEMPLATE,
        revision: 3,
        mode: ProtoAuthoringMode.REFINE,
        prompt: '주장을 줄여 주세요.',
        model: { providerId: 'provider', modelId: 'model' },
        candidateCount: 2,
        stage: 'setting-authoring',
        status: InspectionStatus.PREPARED,
      },
    })
    for (const change of [
      { prompt: '최신 입력' },
      { candidateCount: 4 },
      { revision: 4 },
      { model: { providerId: 'provider', modelId: 'other-model' } },
      { candidateId: 'other-candidate' },
      { operationId: 'other-operation' },
      { mode: 'recommend' as const },
    ]) {
      rendered.rerender(
        <InspectWritingRequestAction
          target={{ ...authoring, ...change }}
          stages={['setting-authoring']}
          defaultStatus="prepared"
        />,
      )
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      rendered.rerender(
        <InspectWritingRequestAction
          target={authoring}
          stages={['setting-authoring']}
          defaultStatus="prepared"
        />,
      )
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
      await screen.findByText('내가 쓴 메모')
    }
    expect(backend.calls.every((call) => call.name === 'GetAuthoringRequestInspection')).toBe(true)
    expect(backend.calls).toHaveLength(8)
  })

  it('does not publish a late response after the current revision is replaced', async () => {
    let release!: (value: RequestInspection) => void
    const pending = new Promise<RequestInspection>((resolve) => {
      release = resolve
    })
    const backend = harness({ read: () => pending })
    const user = userEvent.setup()
    const rendered = render(<InspectWritingRequestAction target={post} />, {
      wrapper: backend.wrapper,
    })
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    await waitFor(() => expect(backend.calls).toHaveLength(1))
    rendered.rerender(
      <InspectWritingRequestAction target={{ ...post, resultRevision: 'result-2' }} />,
    )
    await act(async () => {
      release(inspection())
      await pending
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByText('내가 쓴 메모')).not.toBeInTheDocument()
    expect(backend.calls).toHaveLength(1)
  })

  it('uses server-denied blind availability across a deep identity-bearing payload fixture', async () => {
    const privateFixture = inspection(InspectionStatus.CAPTURED, 'PRIVATE_CALL')
    privateFixture.status = InspectionStatus.UNAVAILABLE
    privateFixture.unavailableReason = 'blind_test_identity_hidden_until_reveal'
    privateFixture.mode = 'PRIVATE_TEMPLATE_MODE'
    privateFixture.fragments[0]!.id = 'PRIVATE_MODEL_RULE'
    privateFixture.fragments[0]!.text =
      'PRIVATE_MODEL PRIVATE_VOICE PRIVATE_TEMPLATE PRIVATE_GUIDELINE'
    privateFixture.fragments[0]!.sourceRefs = ['PRIVATE_SOURCE']
    privateFixture.omissions[0]!.reason = 'PRIVATE_EXCLUSION'
    privateFixture.conditions!.model!.modelId = 'PRIVATE_MODEL'
    privateFixture.output!.schema = 'PRIVATE_RAW_SCHEMA'
    privateFixture.nativeFields[0]!.text = 'PRIVATE_NATIVE_FIELD'
    const backend = harness({ read: () => privateFixture })
    const user = userEvent.setup()
    const target: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'test',
      testId: 'test',
      candidateId: 'candidate',
      revision: 1,
      blind: true,
    }
    render(
      <InspectWritingRequestAction
        target={target}
        defaultStatus="captured"
        stages={['write', 'observe']}
      />,
      { wrapper: backend.wrapper },
    )
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    const sheet = screen.getByRole('dialog')
    expect(await within(sheet).findByText(/블라인드 비교 중에는/)).toBeInTheDocument()
    expect(sheet.textContent).not.toContain('PRIVATE_')
    expect(within(sheet).queryByRole('button', { name: '기술 요청 복사' })).not.toBeInTheDocument()
    expect(backend.calls.map((call) => call.name)).toEqual(['GetWritingTestRequestInspection'])
    expect(privateFixture.status).toBe(InspectionStatus.UNAVAILABLE)
  })

  it('shows absent history as unavailable and keeps raw read errors out of the document', async () => {
    const user = userEvent.setup()
    const absent = harness({
      read: () =>
        create(RequestInspectionSchema, {
          version: 1,
          status: InspectionStatus.UNAVAILABLE,
          stage: 'write',
          unavailableReason: 'capture_missing_stale_or_purged',
        }),
    })
    const rendered = render(
      <InspectWritingRequestAction target={post} defaultStatus="captured" />,
      { wrapper: absent.wrapper },
    )
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    expect(
      await screen.findByText('이 결과에 맞는 전송 기록이 없거나 삭제되었습니다.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('전송 식별자')).not.toBeInTheDocument()
    expect(screen.queryByText('내가 쓴 메모')).not.toBeInTheDocument()
    rendered.unmount()
    const denied = harness({
      read: () => {
        throw new ConnectError('PRIVATE_PROVIDER_RAW_ERROR', Code.PermissionDenied)
      },
    })
    render(<InspectWritingRequestAction target={post} />, { wrapper: denied.wrapper })
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    expect(
      await screen.findByText('현재 요청 정보를 확인할 수 없습니다. 닫았다 다시 열어 주세요.'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/PRIVATE_PROVIDER/)).not.toBeInTheDocument()
  })

  it.each(['day', 'night'])(
    'keeps body-sized explanations and keyboard/mobile controls in the %s theme',
    async (theme) => {
      document.documentElement.dataset.theme = theme
      const user = userEvent.setup()
      const backend = harness()
      render(<InspectWritingRequestAction target={post} stages={['write', 'observe']} />, {
        wrapper: backend.wrapper,
      })
      const opener = screen.getByRole('button', { name: '요청 기술 보기' })
      opener.focus()
      await user.keyboard('{Enter}')
      const sheet = screen.getByRole('dialog')
      await within(sheet).findByText('내가 쓴 메모')
      expect(sheet.className).toContain('w-full')
      expect(sheet.className).toContain('max-h-sheet')
      expect(within(sheet).getByText(/System·User 역할과/)).toHaveClass('text-base')
      expect(within(sheet).getByText(/문자 수와 UTF-8 바이트는/)).toHaveClass('text-base')
      await user.tab()
      expect(within(sheet).getByRole('button', { name: '닫기' })).toHaveFocus()
      await user.tab({ shift: true })
      expect(within(sheet).getByRole('button', { name: '출력 스키마·원문 문법' })).toHaveFocus()
      await user.keyboard('{Escape}')
      expect(opener).toHaveFocus()
      expect(document.body.style.overflow).not.toBe('hidden')
    },
  )

  it('dismisses stale account details and never restores them when authentication returns without an explicit open', async () => {
    const user = userEvent.setup()
    const backend = harness()
    render(<InspectWritingRequestAction target={post} />, { wrapper: backend.wrapper })
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    await screen.findByText('내가 쓴 메모')
    act(() => backend.setOwner('bob'))
    expect(screen.queryByText('내가 쓴 메모')).not.toBeInTheDocument()
    expect(screen.queryByText('요청 정보를 불러오는 중…')).not.toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    act(() => backend.setOwner('alice'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(backend.calls).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
    await screen.findByText('내가 쓴 메모')
    expect(backend.calls).toHaveLength(2)
  })
})
