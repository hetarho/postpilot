import { create } from '@bufbuild/protobuf'
import { createRouterTransport, type Transport } from '@connectrpc/connect'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { useSession } from '@/entities/session'
import type { WritingTest } from '@/entities/writing-test'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import { WritingTestPair, WritingTestChampionOutput } from '@/widgets/writing-test'
import {
  BlockType,
  FragmentAuthorship,
  InspectionRole,
  InspectionStatus,
  PostContentSchema,
  ProtoConfigurationKind,
  RequestInspectionSchema,
  WritingInspectionService,
} from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { renderAppAt } from '@/test/app'
import { resetEditorTest } from '@/test/editor'

afterEach(() => {
  cleanup()
  resetEditorTest()
})

function projection(status: InspectionStatus, stage: string) {
  return create(RequestInspectionSchema, {
    version: 1,
    status,
    stage,
    mode: 'fixture',
    promptVersion: 'prompt-v1',
    schemaVersion: 'schema-v1',
    callId: status === InspectionStatus.CAPTURED ? 'issued-call-7' : '',
    fragments: [
      {
        id: 'instruction',
        role: InspectionRole.SYSTEM,
        authorship: FragmentAuthorship.CODE,
        materialRole: 'instruction',
        text: 'Exact safe instruction',
      },
      {
        id: 'owner',
        role: InspectionRole.USER,
        authorship: FragmentAuthorship.ACCOUNT,
        materialRole: 'source',
        text: 'Frozen owner source',
        sourceRefs: ['memo'],
      },
    ],
    selectedRuleIds: ['exact-rule'],
    output: { name: 'post', version: '1', schema: 'SAFE-CONTRACT' },
    conditions: {
      model: { providerId: 'safe-provider', modelId: 'safe-model' },
      maxCompletionTokens: 123n,
    },
  })
}
function inspectedTransport(
  base: Transport,
  procedures: string[],
  blind: boolean | (() => boolean) = false,
) {
  const inspections = createRouterTransport(({ rpc }) => {
    rpc(WritingInspectionService.method.getPostRequestInspection, (request) => ({
      inspection: projection(request.status, request.stage),
    }))
    rpc(WritingInspectionService.method.getAuthoringRequestInspection, (request) => ({
      inspection: projection(request.status, request.stage),
    }))
    rpc(WritingInspectionService.method.getWritingTestRequestInspection, (request) => ({
      inspection: (typeof blind === 'function' ? blind() : blind)
        ? {
            version: 1,
            status: InspectionStatus.UNAVAILABLE,
            stage: request.stage,
            unavailableReason: 'Identity-bearing inspection is unavailable until reveal.',
          }
        : projection(request.status, request.stage),
    }))
  })
  return new Proxy(base, {
    get(target, property) {
      if (property !== 'unary') return Reflect.get(target, property)
      return async (...args: unknown[]) => {
        const method = args[0] as { name: string; parent: { typeName: string } }
        procedures.push(method.name)
        const selected =
          method.parent.typeName === WritingInspectionService.typeName ? inspections : base
        return Reflect.apply(selected.unary, selected, args)
      }
    },
  })
}
const workCalls = (procedures: string[]) =>
  procedures.filter((name) =>
    /^(Start|Estimate|Save|Create|Patch|SetDefault|Initialize|Cancel|Finalize|Apply|Decide)/.test(
      name,
    ),
  )

it('the real post route opens current/prepared/captured technical reads separately and repeated reads make no work', async () => {
  const user = userEvent.setup()
  const procedures: string[] = []
  const base = createFakeAuthTransport({
    user: { id: 'alice' },
    existingSetup: true,
    posts: {
      posts: [{ slug: 'safe-post', title: 'Stored title', memo: 'Stored memo', status: 'draft' }],
    },
  })
  const view = renderAppAt('/posts/safe-post', { transport: inspectedTransport(base, procedures) })
  const opener = await screen.findByRole('button', { name: '요청 기술 보기' })
  expect(procedures).not.toContain('GetPostRequestInspection')
  const before = workCalls(procedures)
  for (let index = 0; index < 2; index++) {
    await user.click(opener)
    const dialog = await screen.findByRole('dialog', { name: '요청 기술 보기' })
    expect(await within(dialog).findByText('Exact safe instruction')).toBeVisible()
    expect(within(dialog).getByText('Frozen owner source')).toBeVisible()
    if (index === 0) {
      for (const name of ['준비 미리보기', '실제 전송 기록']) {
        await user.click(within(dialog).getByRole('combobox', { name: /^요청 상태/ }))
        await user.click(screen.getByRole('option', { name }))
        await waitFor(() => expect(within(dialog).getByRole('heading', { name })).toBeVisible())
      }
      expect(within(dialog).getByText('issued-call-7')).toBeVisible()
    }
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(opener).toHaveFocus()
  }
  expect(procedures.filter((name) => name === 'GetPostRequestInspection')).toHaveLength(4)
  expect(workCalls(procedures)).toEqual(before)
  view.unmount()
})

it('unsaved owner source edits invalidate the opened technical view and prevent another inspection read', async () => {
  const user = userEvent.setup()
  const procedures: string[] = []
  const base = createFakeAuthTransport({
    user: { id: 'alice' },
    existingSetup: true,
    posts: {
      posts: [{ slug: 'edit-post', title: 'Stored title', memo: 'Stored memo', status: 'draft' }],
    },
  })
  renderAppAt('/posts/edit-post', { transport: inspectedTransport(base, procedures) })
  await user.click(await screen.findByRole('button', { name: '요청 기술 보기' }))
  await screen.findByText('Frozen owner source')
  const before = procedures.filter((name) => name === 'GetPostRequestInspection').length
  fireEvent.change(screen.getByRole('textbox', { name: '메모' }), {
    target: { value: 'Latest unsaved memo' },
  })
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  await user.click(screen.getByRole('button', { name: '요청 기술 보기' }))
  expect(await screen.findByText(/현재 수정한 자료가 아직 저장되지 않아/)).toBeVisible()
  expect(screen.queryByText('Frozen owner source')).not.toBeInTheDocument()
  expect(procedures.filter((name) => name === 'GetPostRequestInspection')).toHaveLength(before)
})

it('setting-authoring composes the current session technical action without exposing grammar in prose or starting another operation', async () => {
  const user = userEvent.setup()
  const procedures: string[] = []
  const base = createFakeAuthTransport({
    user: { id: 'alice' },
    authoring: {
      sessions: [
        {
          id: 'authoring-session',
          kind: ProtoConfigurationKind.POST_TEMPLATE,
          revision: 5,
          phase: 'editing',
          selected: {
            id: 'candidate',
            name: 'Owned draft',
            body: '<write>Private grammar</write>',
          },
        },
      ],
    },
  })
  const transport = inspectedTransport(base, procedures)
  function Host() {
    const { user: owner } = useSession()
    return owner ? (
      <AIAuthoringStudio ownerId={owner.id} kind="post-template" sessionId="authoring-session" />
    ) : null
  }
  render(<Host />, { wrapper: withProviders(transport, createTestQueryClient()) })
  const opener = await screen.findByRole('button', { name: '요청 기술 보기' })
  expect(screen.queryByText('SAFE-CONTRACT')).not.toBeInTheDocument()
  const before = workCalls(procedures)
  await user.click(opener)
  expect(await screen.findByText('Exact safe instruction')).toBeVisible()
  await user.keyboard('{Escape}')
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  expect(workCalls(procedures)).toEqual(before)
  expect(procedures.filter((name) => name === 'GetAuthoringRequestInspection')).toHaveLength(1)
})

it('blind test hosts display the server denied projection and reveal only after the current test revision changes', async () => {
  const user = userEvent.setup()
  const procedures: string[] = []
  const base = createFakeAuthTransport({ user: { id: 'alice' } })
  let test: WritingTest = {
    id: 'blind-test',
    revision: 1,
    factor: 'model',
    modelStage: 'write',
    count: 2,
    status: 'review',
    sourcePostSlug: '',
    jobId: '',
    candidates: [0, 1].map((index) => ({
      id: `opaque-${index}`,
      status: 'succeeded',
      displayLabel: '',
      output: create(PostContentSchema, {
        title: `Post ${index}`,
        blocks: [{ type: BlockType.TEXT, content: 'Complete writing' }],
      }),
    })),
    matches: [
      {
        id: 'match',
        round: 1,
        index: 0,
        leftCandidateId: 'opaque-0',
        rightCandidateId: 'opaque-1',
        winnerCandidateId: '',
      },
    ],
    winnerCandidateId: '',
    publications: [],
    revealed: false,
    createdAt: '',
    updatedAt: '',
    contentExpiresAt: '2099-01-01T00:00:00Z',
    fictional: false,
    confirmedCredits: 0,
    reservedCredits: 0,
    targetLanguage: 'ko',
  }
  const transport = inspectedTransport(base, procedures, () => !test.revealed)
  function Host() {
    const { user: owner } = useSession()
    return owner ? (
      test.status === 'completed' ? (
        <WritingTestChampionOutput ownerId={owner.id} test={test} />
      ) : (
        <WritingTestPair
          ownerId={owner.id}
          test={test}
          match={test.matches[0]!}
          onWinner={() => undefined}
          onShowCandidate={() => undefined}
          onReading={() => undefined}
        />
      )
    ) : null
  }
  const view = render(<Host />, { wrapper: withProviders(transport, createTestQueryClient()) })
  const openers = await screen.findAllByRole('button', { name: '요청 기술 보기' })
  const before = workCalls(procedures)
  await user.click(openers[0]!)
  expect(
    await screen.findByText('Identity-bearing inspection is unavailable until reveal.'),
  ).toBeVisible()
  expect(
    screen.queryByText(/safe-provider|safe-model|Exact safe instruction|Frozen owner source/),
  ).not.toBeInTheDocument()
  await user.keyboard('{Escape}')
  test = {
    ...test,
    status: 'completed',
    revision: 2,
    revealed: true,
    winnerCandidateId: 'opaque-0',
  }
  view.rerender(<Host />)
  await user.click(await screen.findByRole('button', { name: '요청 기술 보기' }))
  expect(await screen.findByText('issued-call-7')).toBeVisible()
  expect(screen.getByText('safe-provider/safe-model')).toBeVisible()
  expect(workCalls(procedures)).toEqual(before)
  expect(procedures.filter((name) => name === 'GetWritingTestRequestInspection')).toHaveLength(2)
})
