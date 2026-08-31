import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
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
  status: 'review',
  postSlug: 'post',
  voiceId: '',
  purposeName: '',
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
      failure: undefined,
      modelLabel: '',
    },
  ],
}

function actionSet() {
  return {
    choose: vi.fn(),
    decideWrite: vi.fn().mockResolvedValue({}),
    useSingle: vi.fn(),
    dismiss: vi.fn(),
    retry: vi.fn(),
    apply: vi.fn(),
    adopt: vi.fn(),
    isPending: false,
    failure: undefined,
  }
}

function renderActions(
  experiment = base,
  voices: FakeVoiceRow[] = [{ id: 'voice-default', name: '기본 말투', isDefault: true }],
) {
  const backend = createFakeAuthBackend({ user: { id: 'alice' }, voice: { voices } })
  render(<ExperimentActions experiment={experiment} activeCandidateId="left" />, {
    wrapper: withProviders(backend.transport, createTestQueryClient()),
  })
}

beforeEach(() => mocks.useExperimentActions.mockReset())

it('offers direct apply-only and apply-and-adopt decisions for a ready write pair', async () => {
  const actions = actionSet()
  mocks.useExperimentActions.mockReturnValue(actions)
  renderActions()
  const user = userEvent.setup()
  expect(screen.queryByRole('button', { name: '이 결과로 선택' })).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '결과 적용' }))
  expect(actions.decideWrite).toHaveBeenCalledWith('left', false)
  await user.click(screen.getByRole('button', { name: '결과 적용하고 활성 모델로 변경' }))
  expect(actions.decideWrite).toHaveBeenCalledWith('left', true)
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
