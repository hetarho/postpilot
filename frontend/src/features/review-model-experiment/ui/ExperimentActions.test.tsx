import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { createFakeAuthBackend, createTestQueryClient, withProviders } from '@/test/session'
import { ExperimentActions } from './ExperimentActions'

const mocks = vi.hoisted(() => ({ useExperimentActions: vi.fn() }))
vi.mock('@/entities/model-experiment', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/model-experiment')>()),
  useExperimentActions: mocks.useExperimentActions,
}))

const base: ModelExperiment = {
  id: 'experiment-1', stage: 'write', status: 'review', postSlug: 'post', jobId: 'job',
	winnerCandidateId: '', outcome: '', applyError: '', appliedAt: '', adoptionRequested: false,
	adoptionError: '', adoptedAt: '',
  createdAt: '', finishedAt: '', decidedAt: '', revealed: false,
  candidates: [
    { id: 'left', displaySide: 'left', status: 'succeeded', output: { kind: 'write', content: { $typeName: 'postpilot.v1.PostContent', title: 'A', summary: '', tags: [], blocks: [] } }, error: '', modelLabel: '' },
    { id: 'right', displaySide: 'right', status: 'succeeded', output: { kind: 'write', content: { $typeName: 'postpilot.v1.PostContent', title: 'B', summary: '', tags: [], blocks: [] } }, error: '', modelLabel: '' },
  ],
}

function actionSet() {
  return {
    choose: vi.fn(), decideWrite: vi.fn().mockResolvedValue({}), useSingle: vi.fn(), dismiss: vi.fn(),
    retry: vi.fn(), apply: vi.fn(), adopt: vi.fn(), isPending: false, error: undefined,
  }
}

function renderActions(experiment = base) {
  const backend = createFakeAuthBackend({ user: { id: 'alice' } })
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
    adoptionError: 'temporary selection failure',
  })
  expect(screen.getByText(/결과는 적용했지만 활성 작성 모델은 변경하지 못했어요/)).toBeInTheDocument()
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
		applyError: 'temporary post failure',
		adoptionRequested: true,
	})

	await userEvent.click(screen.getByRole('button', { name: '적용 다시 시도' }))
	expect(actions.decideWrite).toHaveBeenCalledWith('left', true)
})
