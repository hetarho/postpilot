import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import { ContentRevisionConflictError } from '@/entities/post'
import { voiceProfileQueryKey } from '@/entities/voice'
import { ProtoGuidelineScope, Stage } from '@/shared/api'
import { REVISION_INSTRUCTION_MAX_CHARS } from '@/shared/config'
import type { FakeGuidelinesOptions } from '@/test/guidelines'
import type { FakeRevisionStart } from '@/test/jobs'
import { connectAppError } from '@/test/app-error'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { ReviseForm } from './ReviseForm'

const activeJob: GenerationJob = {
  id: 'active',
  kind: 'revise',
  status: 'running',
  stage: 'write',
  progressDone: 0,
  progressTotal: 1,
  failure: undefined,
  postSlug: 'post',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '',
  updatedAt: '',
  targetLanguage: 'ko',
}

function renderForm({
  selected = true,
  active,
  revisions = [],
  beforeStart,
  purpose,
  guidelines,
  calls,
}: {
  selected?: boolean
  active?: GenerationJob
  revisions?: FakeRevisionStart[]
  beforeStart?: () => Promise<void>
  purpose?: { id: string; name: string }
  guidelines?: FakeGuidelinesOptions
  calls?: string[]
} = {}) {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'writer' }],
      selections: selected
        ? [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }]
        : [],
    },
    jobs: { revisions, startJobId: 'revision-new' },
    guidelines,
  })
  const onStarted = vi.fn()
  const queryClient = createTestQueryClient()
  render(
    <ReviseForm
      ownerId="alice"
      postSlug="post"
      voice={{ id: 'voice-a', deleted: false }}
      activeJob={active}
      purpose={purpose}
      onStarted={onStarted}
      beforeStart={beforeStart}
    />,
    {
      wrapper: withProviders(transport, queryClient),
    },
  )
  return { onStarted, queryClient, transport }
}

const doneJob: GenerationJob = { ...activeJob, id: 'done', status: 'done' }

it('requires an instruction and the explicit write selection', async () => {
  renderForm({ selected: false })

  expect(await screen.findByText('작성 모델을 선택하세요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
  expect(screen.getByLabelText('수정 요청')).toHaveAttribute(
    'maxlength',
    String(REVISION_INSTRUCTION_MAX_CHARS),
  )
})

it('preserves a structured failure from the prerequisite content save', async () => {
  renderForm({
    beforeStart: () =>
      Promise.reject(connectAppError('POST_CONTENT_INVALID', Code.InvalidArgument)),
  })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('수정 요청'), '존댓말로')
  await user.click(screen.getByRole('button', { name: '수정' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('글 내용을 확인해 주세요.')
  expect(screen.queryByText('private backend prose')).not.toBeInTheDocument()
})

it('keeps a content revision conflict as contextual recovery guidance', async () => {
  renderForm({ beforeStart: () => Promise.reject(new ContentRevisionConflictError()) })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('수정 요청'), '존댓말로')
  await user.click(screen.getByRole('button', { name: '수정' }))

  expect(await screen.findByRole('alert')).toHaveTextContent(
    '다른 화면에서 글이 바뀌었어요. 이 화면을 새로고침한 뒤 다시 수정해 주세요.',
  )
})

it('stays disabled while another job is active', async () => {
  renderForm({ active: activeJob })

  expect(await screen.findByText('다른 작업이 진행 중이에요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
})

it('starts a revision with its instruction, rule flag, and selected write model', async () => {
  const revisions: FakeRevisionStart[] = []
  const { onStarted, queryClient, transport } = renderForm({ revisions })
  const ownProfile = voiceProfileQueryKey(transport, 'alice', 'voice-a')
  const siblingProfile = voiceProfileQueryKey(transport, 'alice', 'voice-b')
  queryClient.setQueryData(ownProfile, { profile: 'own' })
  queryClient.setQueryData(siblingProfile, { profile: 'sibling' })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('수정 요청'), '  존댓말로  ')
  await user.click(screen.getByRole('checkbox', { name: '이 요청을 규칙으로 저장' }))
  const button = screen.getByRole('button', { name: '수정' })
  await waitFor(() => expect(button).toBeEnabled())

  await user.click(button)

  await waitFor(() => expect(onStarted).toHaveBeenCalledWith('revision-new'))
  expect(queryClient.getQueryState(ownProfile)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(siblingProfile)?.isInvalidated).toBe(false)
  expect(revisions).toEqual([
    {
      postSlug: 'post',
      instruction: '존댓말로',
      saveAsRule: true,
      writeModel: { providerId: 'openrouter', modelId: 'writer' },
    },
  ])
})

it('does not offer it after a completed generate job', async () => {
  renderForm({ active: { ...doneJob, kind: 'generate' } })
  await screen.findByLabelText('수정 요청')
  expect(screen.queryByRole('button', { name: '지침으로 저장' })).not.toBeInTheDocument()
})

// Plan 16 A11: the capture appears only once a revision has FINISHED — the instruction is worth
// saving as a rule after the user has seen what it did.
it('offers 지침으로 저장 only after a completed revision', async () => {
  const user = userEvent.setup()
  renderForm()
  await user.type(screen.getByLabelText('수정 요청'), '무인 매장이니까 주인 얘기 빼줘')
  expect(screen.queryByRole('button', { name: '지침으로 저장' })).not.toBeInTheDocument()

  renderForm({ active: doneJob })
  await waitFor(() =>
    expect(screen.getAllByRole('button', { name: '지침으로 저장' })[0]).toBeInTheDocument(),
  )
})

it('does not offer it after a failed revision', async () => {
  renderForm({ active: { ...activeJob, status: 'failed' } })
  await screen.findByLabelText('수정 요청')
  expect(screen.queryByRole('button', { name: '지침으로 저장' })).not.toBeInTheDocument()
})

// Plan 16 A11: the dialog is seeded with the instruction, editable before saving, and offers 전역
// by default plus the post's purpose when it has one.
it('seeds the dialog with the instruction and saves it scoped to the post purpose', async () => {
  const user = userEvent.setup()
  const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
  const calls: string[] = []
  renderForm({
    active: doneJob,
    purpose: { id: 'purpose-review', name: '무인가게 리뷰' },
    guidelines: { creates },
    calls,
  })
  await user.type(screen.getByLabelText('수정 요청'), '무인 매장이니까 주인 얘기 빼줘')
  await user.click(screen.getByRole('button', { name: '지침으로 저장' }))

  const dialog = await screen.findByRole('dialog')
  const field = within(dialog).getByLabelText('지침')
  expect(field).toHaveValue('무인 매장이니까 주인 얘기 빼줘')
  // 전역 is the default; the post's purpose is offered beside it, by name.
  expect(within(dialog).getByRole('tab', { name: '전역', selected: true })).toBeInTheDocument()

  // Generalized before saving, which is the whole point of letting the user edit it.
  await user.clear(field)
  await user.type(field, '무인 매장 글에서 주인 이야기를 쓰지 않기')
  await user.click(within(dialog).getByRole('tab', { name: /무인가게 리뷰/ }))
  await user.click(within(dialog).getByRole('button', { name: '저장' }))

  await waitFor(() => expect(creates).toHaveLength(1))
  expect(creates[0]).toEqual({
    text: '무인 매장 글에서 주인 이야기를 쓰지 않기',
    scope: ProtoGuidelineScope.PURPOSES,
    purposeIds: ['purpose-review'],
  })
  expect(await screen.findByText('지침으로 저장했어요.')).toBeInTheDocument()
  // A15: the capture is a plain create — it starts nothing and calls no provider ([I5]).
  expect(calls.filter((call) => call === 'StartRevision')).toEqual([])
})

// A post with no purpose gets no scope choice at all: 전역 is the only shape available.
it('offers no purpose scope when the post has none', async () => {
  const user = userEvent.setup()
  const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
  renderForm({ active: doneJob, guidelines: { creates } })
  await user.type(screen.getByLabelText('수정 요청'), '문장을 짧게 해줘')
  await user.click(screen.getByRole('button', { name: '지침으로 저장' }))

  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).queryByRole('tab')).not.toBeInTheDocument()
  await user.click(within(dialog).getByRole('button', { name: '저장' }))

  await waitFor(() => expect(creates).toHaveLength(1))
  expect(creates[0]).toEqual({
    text: '문장을 짧게 해줘',
    scope: ProtoGuidelineScope.GLOBAL,
    purposeIds: [],
  })
})

// A11: AlreadyExists is information — the rule the user wanted is already saved.
it('reports an exact duplicate as already saved rather than as a failure', async () => {
  const user = userEvent.setup()
  renderForm({ active: doneJob, guidelines: { createDuplicates: true } })
  await user.type(screen.getByLabelText('수정 요청'), '주인 얘기 빼줘')
  await user.click(screen.getByRole('button', { name: '지침으로 저장' }))

  const dialog = await screen.findByRole('dialog')
  await user.click(within(dialog).getByRole('button', { name: '저장' }))

  expect(await screen.findByText('이미 같은 지침이 있어요.')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
