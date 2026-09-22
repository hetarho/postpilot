import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import type { FakePostRow } from '@/test/posts'
import { createFakeAuthBackend, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeVoiceRow } from '@/test/voice'
import { ExperimentActions } from './ExperimentActions'

const mocks = vi.hoisted(() => ({ useExperimentActions: vi.fn() }))
vi.mock('@/entities/model-experiment', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/model-experiment')>()),
  useExperimentActions: mocks.useExperimentActions,
}))

const base: ModelExperiment = {
  id: 'experiment-1',
  stage: 'write',
  origin: 'editor',
  status: 'review',
  postSlug: 'post',
  voiceId: '',
  templateName: '',
  jobId: 'job',
  winnerCandidateId: '',
  outcome: '',
  applyFailure: undefined,
  appliedAt: '',
  adoptionRequested: false,
  adoptionFailure: undefined,
  adoptedAt: '',
  createdAt: '',
  finishedAt: '',
  decidedAt: '',
  revealed: false,
  targetLanguage: 'ko',
  candidates: [
    {
      id: 'left',
      displaySide: 'left',
      status: 'succeeded',
      output: {
        kind: 'write',
        content: {
          $typeName: 'postpilot.v1.PostContent',
          title: 'A',
          summary: '',
          tags: [],
          blocks: [],
        },
      },
      badges: [],
      otherNote: '',
      failure: undefined,
      modelLabel: '',
    },
    {
      id: 'right',
      displaySide: 'right',
      status: 'succeeded',
      output: {
        kind: 'write',
        content: {
          $typeName: 'postpilot.v1.PostContent',
          title: 'B',
          summary: '',
          tags: [],
          blocks: [],
        },
      },
      badges: [],
      otherNote: '',
      failure: undefined,
      modelLabel: '',
    },
  ],
}

function actionSet() {
  // Every action resolves: the bar awaits what it calls to clear its pressed state, so a
  // mock returning undefined fails inside React's event handler rather than in the assertion.
  return {
    choose: vi.fn().mockResolvedValue({}),
    decideWrite: vi.fn().mockResolvedValue({}),
    useSingle: vi.fn().mockResolvedValue({}),
    dismiss: vi.fn().mockResolvedValue({}),
    retry: vi.fn().mockResolvedValue({}),
    apply: vi.fn().mockResolvedValue({}),
    adopt: vi.fn().mockResolvedValue({}),
    isPending: false,
    badges: [],
    otherNote: '',
    failure: undefined,
  }
}

function renderActions(
  experiment = base,
  voices: FakeVoiceRow[] = [{ id: 'voice-default', name: '기본 말투', isDefault: true }],
  posts: FakePostRow[] = [{ slug: 'post', status: 'draft' }],
) {
  const backend = createFakeAuthBackend({
    user: { id: 'alice' },
    voice: { voices },
    posts: { posts },
  })
  return render(<ExperimentActions experiment={experiment} activeCandidateId="left" />, {
    wrapper: withProviders(backend.transport, createTestQueryClient()),
  })
}

/** The sheet's confirm, told apart from the dock button that opened it: both carry the same
 *  label, and only one of them is inside the dialog. */
async function confirmVerdict(user: ReturnType<typeof userEvent.setup>, label: string) {
  const sheet = await screen.findByRole('dialog')
  await user.click(within(sheet).getByRole('button', { name: label }))
}

beforeEach(() => mocks.useExperimentActions.mockReset())

// Both committing write actions pass through the one sheet, and the badges the sheet
// collected ride along with the verdict they explain (MODEL-61).
it('offers direct apply-only and apply-and-adopt decisions for a ready write pair', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions()
  const user = userEvent.setup()
  expect(screen.queryByRole('button', { name: '이 결과로 선택' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '결과 적용' }))
  await confirmVerdict(user, '결과 적용')
  expect(actions.decideWrite).toHaveBeenCalledWith('left', false, [])
  await user.click(screen.getByRole('button', { name: '결과 적용하고 활성 모델로 변경' }))
  await confirmVerdict(user, '결과 적용하고 활성 모델로 변경')
  expect(actions.decideWrite).toHaveBeenCalledWith('left', true, [])
})

it('reports applied content separately and retries only model adoption', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions({
    ...base,
    status: 'decided',
    winnerCandidateId: 'left',
    revealed: true,
    appliedAt: '2026-08-30T00:00:00Z',
    adoptionFailure: { reason: 'MODEL_UNAVAILABLE', params: {} },
  })
  expect(
    screen.getByText(/결과는 적용했지만 활성 작성 모델은 변경하지 못했어요/),
  ).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '활성 모델 변경 다시 시도' }))
  expect(actions.decideWrite).toHaveBeenCalledWith('left', true)
  expect(actions.apply).not.toHaveBeenCalled()
})

it('preserves apply-and-adopt intent when content application itself needs a retry', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions({
    ...base,
    status: 'decided',
    winnerCandidateId: 'left',
    revealed: true,
    applyFailure: { reason: 'POST_BUSY', params: {} },
    adoptionRequested: true,
  })

  await userEvent.click(screen.getByRole('button', { name: '적용 다시 시도' }))
  expect(actions.decideWrite).toHaveBeenCalledWith('left', true)
})

it('blocks provider and apply work when the experiment voice is deleted', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions(
    {
      ...base,
      stage: 'analyze',
      status: 'decided',
      voiceId: 'voice-old',
      winnerCandidateId: 'left',
      revealed: true,
    },
    [
      { id: 'voice-default', name: '기본 말투', isDefault: true },
      { id: 'voice-old', name: '옛 말투', deleted: true },
    ],
  )

  expect(await screen.findByText(/삭제되었거나 찾을 수 없는 말투/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '결과 적용' })).toBeDisabled()
  expect(actions.apply).not.toHaveBeenCalled()
})

const labPair: ModelExperiment = { ...base, origin: 'lab' }
const decidedLabPair: ModelExperiment = {
  ...labPair,
  status: 'decided',
  winnerCandidateId: 'left',
  revealed: true,
}

it('offers the lab only a pick, and offers the editor no pick at all', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions(labPair)
  expect(screen.queryByRole('button', { name: '결과 적용' })).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: '결과 적용하고 활성 모델로 변경' }),
  ).not.toBeInTheDocument()
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: '이 결과로 선택' }))
  // A badge chosen in the sheet reaches the verdict it explains.
  await user.click(screen.getAllByRole('button', { name: '속도가 빨라요' })[0])
  await confirmVerdict(user, '이 결과로 선택')
  expect(actions.choose).toHaveBeenCalledWith('left', [
    { candidateId: 'left', badges: ['fast'], otherNote: '' },
  ])
  expect(actions.decideWrite).not.toHaveBeenCalled()
})

it('offers a decided lab pick the model adoption and, on a draft, the content application', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions(decidedLabPair)
  await userEvent.click(await screen.findByRole('button', { name: '결과 적용' }))
  expect(actions.apply).toHaveBeenCalled()
  await userEvent.click(screen.getByRole('button', { name: '활성 모델로 사용' }))
  expect(actions.adopt).toHaveBeenCalled()
  expect(actions.decideWrite).not.toHaveBeenCalled()
})

it('withholds the content application from a finalized post and keeps the adoption', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions(decidedLabPair, undefined, [{ slug: 'post', status: 'finalized' }])
  expect(await screen.findByRole('button', { name: '활성 모델로 사용' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '결과 적용' })).not.toBeInTheDocument()
})

it('withholds the content application when the post it ran on is gone', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions(decidedLabPair, undefined, [])
  expect(await screen.findByRole('button', { name: '활성 모델로 사용' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '결과 적용' })).not.toBeInTheDocument()
})

it('keeps the survivor of a half-failed comparison usable from either surface', async () => {
  for (const experiment of [base, labPair]) {
    mocks.useExperimentActions.mockReset()
    const actions = actionSet()
    mocks.useExperimentActions.mockReturnValue(actions)
    const { unmount } = renderActions({
      ...experiment,
      status: 'partial',
      candidates: [
        experiment.candidates[0],
        { ...experiment.candidates[1], status: 'failed', output: undefined },
      ],
    })
    await userEvent.click(screen.getByRole('button', { name: '이 결과만 사용' }))
    expect(actions.useSingle).toHaveBeenCalledWith('left')
    unmount()
  }
})
