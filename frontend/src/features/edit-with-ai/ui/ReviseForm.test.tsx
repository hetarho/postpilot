import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import { ContentRevisionConflictError } from '@/entities/post'
import { voiceAnalysisQueryKey } from '@/entities/voice'
import { ProtoGuidelineScope, Stage } from '@/shared/api'
import { REVISION_INSTRUCTION_MAX_CHARS } from '../config'
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
  template,
  guidelines,
  calls,
}: {
  selected?: boolean
  active?: GenerationJob
  revisions?: FakeRevisionStart[]
  beforeStart?: () => Promise<void>
  template?: { id: string; name: string }
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
      voice={{ deleted: false, made: true }}
      activeJob={active}
      template={template}
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

  expect(
    await screen.findByText('글 생성 단계의 글쓰기 옵션에서 작성 모델을 선택하세요.'),
  ).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
  expect(screen.getByLabelText('수정 요청을 입력하세요')).toHaveAttribute(
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
  await user.type(await screen.findByLabelText('수정 요청을 입력하세요'), '존댓말로')
  await user.click(screen.getByRole('button', { name: '수정' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('글 내용을 확인해 주세요.')
  expect(screen.queryByText('private backend prose')).not.toBeInTheDocument()
})

it('keeps a content revision conflict as contextual recovery guidance', async () => {
  renderForm({ beforeStart: () => Promise.reject(new ContentRevisionConflictError()) })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('수정 요청을 입력하세요'), '존댓말로')
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

// POST-56: a revision teaches no voice anything, so the form offers no 규칙으로 저장 and the start
// leaves every voice profile it could have touched as it was.
it('starts a revision with its instruction and selected write model, and nothing else', async () => {
  const revisions: FakeRevisionStart[] = []
  const { onStarted, queryClient, transport } = renderForm({ revisions })
  const profile = voiceAnalysisQueryKey(transport, 'alice', 'voice-a')
  queryClient.setQueryData(profile, { profile: 'own' })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('수정 요청을 입력하세요'), '  존댓말로  ')
  // The secondary row is open now, and it holds the counter and nothing to tick.
  expect(screen.getByText(`8/${REVISION_INSTRUCTION_MAX_CHARS}`)).toBeInTheDocument()
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(screen.queryByText(/규칙으로 저장/)).not.toBeInTheDocument()
  const button = screen.getByRole('button', { name: '수정' })
  await waitFor(() => expect(button).toBeEnabled())

  await user.click(button)

  await waitFor(() => expect(onStarted).toHaveBeenCalledWith('revision-new'))
  expect(queryClient.getQueryState(profile)?.isInvalidated).toBe(false)
  expect(revisions).toEqual([
    {
      postSlug: 'post',
      instruction: '존댓말로',
      writeModel: { providerId: 'openrouter', modelId: 'writer' },
    },
  ])
})

it('does not offer it after a completed generate job', async () => {
  renderForm({ active: { ...doneJob, kind: 'generate' } })
  await screen.findByLabelText('수정 요청을 입력하세요')
  expect(screen.queryByRole('button', { name: '지침으로 저장' })).not.toBeInTheDocument()
})

// GUIDE-21: the capture appears only once a revision has FINISHED — the instruction is worth
// saving as a rule after the user has seen what it did.
it('offers 지침으로 저장 only after a completed revision', async () => {
  const user = userEvent.setup()
  renderForm()
  await user.type(screen.getByLabelText('수정 요청을 입력하세요'), '무인 매장이니까 주인 얘기 빼줘')
  expect(screen.queryByRole('button', { name: '지침으로 저장' })).not.toBeInTheDocument()

  cleanup()
  renderForm({ active: doneJob })
  // The secondary row collapses while the field is empty and unfocused, so the instruction the
  // revision actually ran with is what puts the capture on screen.
  await user.type(
    await screen.findByLabelText('수정 요청을 입력하세요'),
    '무인 매장이니까 주인 얘기 빼줘',
  )
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '지침으로 저장' })).toBeInTheDocument(),
  )
})

// A11: the dock is over the draft the whole time, so the row that is not being used is height
// taken from the thing the screen is for.
it('collapses its secondary controls while the instruction is empty and unfocused', async () => {
  const user = userEvent.setup()
  renderForm()

  const field = await screen.findByLabelText('수정 요청을 입력하세요')
  const counter = () => screen.queryByText(`0/${REVISION_INSTRUCTION_MAX_CHARS}`)
  expect(counter()).not.toBeInTheDocument()

  await user.click(field)
  expect(counter()).toBeInTheDocument()

  // One Tab past the last control in the form lands on the body, which is a focus move OUT.
  await user.tab()
  await waitFor(() => expect(counter()).not.toBeInTheDocument())
})

// A11: a running revision keeps the row open with nothing typed, because that is exactly when its
// state is worth reading.
it('keeps the secondary controls open while a revision is running', async () => {
  renderForm({ active: activeJob })
  expect(await screen.findByText(`0/${REVISION_INSTRUCTION_MAX_CHARS}`)).toBeInTheDocument()
})

it('does not offer it after a failed revision', async () => {
  renderForm({ active: { ...activeJob, status: 'failed' } })
  await screen.findByLabelText('수정 요청을 입력하세요')
  expect(screen.queryByRole('button', { name: '지침으로 저장' })).not.toBeInTheDocument()
})

// GUIDE-21: the dialog is seeded with the instruction, editable before saving, and offers 전역
// by default plus the post's template when it has one.
it('seeds the dialog with the instruction and saves it scoped to the post template', async () => {
  const user = userEvent.setup()
  const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
  const calls: string[] = []
  renderForm({
    active: doneJob,
    template: { id: 'template-review', name: '무인가게 리뷰' },
    guidelines: { creates },
    calls,
  })
  await user.type(screen.getByLabelText('수정 요청을 입력하세요'), '무인 매장이니까 주인 얘기 빼줘')
  await user.click(screen.getByRole('button', { name: '지침으로 저장' }))

  const dialog = await screen.findByRole('dialog')
  const field = within(dialog).getByLabelText('지침')
  expect(field).toHaveValue('무인 매장이니까 주인 얘기 빼줘')
  // 전역 is the default; the post's template is offered beside it, by name — and nothing else: the
  // capture offers no 분야 scope (GUIDE-21).
  expect(within(dialog).getByRole('tab', { name: '전역', selected: true })).toBeInTheDocument()
  expect(within(dialog).getAllByRole('tab')).toHaveLength(2)

  // Generalized before saving, which is the whole point of letting the user edit it.
  await user.clear(field)
  await user.type(field, '무인 매장 글에서 주인 이야기를 쓰지 않기')
  await user.click(within(dialog).getByRole('tab', { name: /무인가게 리뷰/ }))
  // GUIDE-46: the optional title opens empty and rides the create when one is typed.
  const title = within(dialog).getByLabelText('제목')
  expect(title).toHaveValue('')
  await user.type(title, '주인 이야기 빼기')
  await user.click(within(dialog).getByRole('button', { name: '저장' }))

  await waitFor(() => expect(creates).toHaveLength(1))
  expect(creates[0]).toEqual({
    title: '주인 이야기 빼기',
    text: '무인 매장 글에서 주인 이야기를 쓰지 않기',
    scope: ProtoGuidelineScope.TEMPLATES,
    templateIds: ['template-review'],
    fields: [],
  })
  expect(await screen.findByText('지침으로 저장했어요.')).toBeInTheDocument()
  // A15: the capture is a plain create — it starts nothing and calls no provider ([I5]).
  expect(calls.filter((call) => call === 'StartRevision')).toEqual([])
})

// A post with no template gets no scope choice at all: 전역 is the only shape available.
it('offers no template scope when the post has none', async () => {
  const user = userEvent.setup()
  const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
  renderForm({ active: doneJob, guidelines: { creates } })
  await user.type(screen.getByLabelText('수정 요청을 입력하세요'), '문장을 짧게 해줘')
  await user.click(screen.getByRole('button', { name: '지침으로 저장' }))

  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).queryByRole('tab')).not.toBeInTheDocument()
  await user.click(within(dialog).getByRole('button', { name: '저장' }))

  await waitFor(() => expect(creates).toHaveLength(1))
  expect(creates[0]).toEqual({
    text: '문장을 짧게 해줘',
    scope: ProtoGuidelineScope.GLOBAL,
    templateIds: [],
    fields: [],
  })
})

// A11: AlreadyExists is information — the rule the user wanted is already saved.
it('reports an exact duplicate as already saved rather than as a failure', async () => {
  const user = userEvent.setup()
  renderForm({ active: doneJob, guidelines: { createDuplicates: true } })
  await user.type(screen.getByLabelText('수정 요청을 입력하세요'), '주인 얘기 빼줘')
  await user.click(screen.getByRole('button', { name: '지침으로 저장' }))

  const dialog = await screen.findByRole('dialog')
  await user.click(within(dialog).getByRole('button', { name: '저장' }))

  expect(await screen.findByText('이미 같은 지침이 있어요.')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
