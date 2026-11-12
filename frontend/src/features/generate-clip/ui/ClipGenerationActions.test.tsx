import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import {
  emptyClipProject,
  type ClipProject,
  type ClipQuote,
  type ReadyClipBatch,
} from '@/entities/clip-project'
import { ClipGenerationActions } from './ClipGenerationActions'

const state = vi.hoisted(() => ({ expired: false, quoted: vi.fn() }))
vi.mock('@/entities/plan', () => ({
  useMyPlan: () => ({ myPlan: { balance: { credits: 100, unlimited: false, renewsAt: '' } } }),
}))
vi.mock('../api/useClipQuote', () => ({
  useClipQuote: (...args: unknown[]) => {
    state.quoted(...args)
    return {
      quote: state.expired ? undefined : quote,
      expired: state.expired,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
    }
  },
}))
const quote: ClipQuote = {
  quoteId: 'approved-quote',
  maxCredits: 12,
  expiresAt: '2099-01-01T00:00:00Z',
  binding: 'frozen',
  cancellationPolicy: { version: 1, numerator: 1, denominator: 2, rounding: 'ceil' },
  calls: [{ label: 'flow', calls: 1 }],
}
const project: ClipProject = {
  ...emptyClipProject(),
  id: 'clip',
  title: 'saved',
  createdAt: '',
  updatedAt: '',
  editPlanRevision: 0,
  renderedPlanRevision: 0,
}
const batch: ReadyClipBatch = {
  id: 'batch',
  projectId: 'clip',
  state: 'ready',
  expiresAt: '2099-01-01T00:00:00Z',
  sources: [],
}
const model = { providerId: 'provider', modelId: 'model' }
afterEach(() => {
  state.quoted.mockClear()
  state.expired = false
})

it.each(['storyline', 'generate'] as const)(
  'quotes %s only on its explicit action and approves the exact ceiling',
  async (mode) => {
    const approve = vi.fn()
    const user = userEvent.setup()
    render(
      <ClipGenerationActions
        stepBar
        ownerId="owner"
        project={project}
        batch={batch}
        observe={model}
        write={model}
        observeStatus="eligible"
        ready
        pending={false}
        onApprove={approve}
      />,
    )
    expect(state.quoted).not.toHaveBeenCalled()
    const name = mode === 'storyline' ? '스토리라인 먼저' : '바로 만들기'
    await user.click(screen.getByRole('button', { name }))
    expect(state.quoted).toHaveBeenLastCalledWith(
      'owner',
      project,
      batch,
      model,
      model,
      'eligible',
      mode,
    )
    expect(approve).not.toHaveBeenCalled()
    const dialog = within(screen.getByRole('dialog', { name }))
    await user.click(
      dialog.getByRole('button', {
        name:
          mode === 'storyline'
            ? '최대 12 크레딧 · 승인하고 스토리라인 만들기'
            : '최대 12 크레딧 · 승인하고 생성',
      }),
    )
    expect(approve).toHaveBeenCalledExactlyOnceWith(mode, quote)
  },
)

it('retains the open approval through current-input props and never submits the form from a trigger', async () => {
  const approve = vi.fn(),
    submit = vi.fn()
  const fixture = (value: ClipProject) => (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
    >
      <ClipGenerationActions
        stepBar
        ownerId="owner"
        project={value}
        batch={batch}
        observe={model}
        write={model}
        observeStatus="eligible"
        ready
        pending={false}
        onApprove={approve}
      />
    </form>
  )
  const view = render(fixture(project))
  await userEvent.click(screen.getByRole('button', { name: '바로 만들기' }))
  const dialog = screen.getByRole('dialog', { name: '바로 만들기' })
  view.rerender(fixture({ ...project, title: 'latest local save' }))
  expect(screen.getByRole('dialog', { name: '바로 만들기' })).toBe(dialog)
  expect(approve).not.toHaveBeenCalled()
  expect(submit).not.toHaveBeenCalled()
})

it('keeps unavailable readiness closed with an actionable description and no quote', () => {
  render(
    <>
      <p id="reason">모델 선택을 확인해 주세요.</p>
      <ClipGenerationActions
        stepBar
        describedBy="reason"
        ownerId="owner"
        project={project}
        batch={batch}
        observe={model}
        write={model}
        observeStatus={undefined}
        ready={false}
        pending={false}
        onApprove={vi.fn()}
      />
    </>,
  )
  const button = screen.getByRole('button', { name: '바로 만들기' })
  expect(button).toBeDisabled()
  expect(button).toHaveAccessibleDescription('모델 선택을 확인해 주세요.')
  fireEvent.click(button)
  expect(state.quoted).not.toHaveBeenCalled()
})

it('refuses an expired ceiling without invoking approval', async () => {
  state.expired = true
  const approve = vi.fn()
  render(
    <ClipGenerationActions
      stepBar
      ownerId="owner"
      project={project}
      batch={batch}
      observe={model}
      write={model}
      observeStatus="eligible"
      ready
      pending={false}
      onApprove={approve}
    />,
  )
  await userEvent.click(screen.getByRole('button', { name: '바로 만들기' }))
  expect(
    within(screen.getByRole('dialog', { name: '바로 만들기' })).getByRole('button', {
      name: '바로 만들기',
    }),
  ).toBeDisabled()
  expect(approve).not.toHaveBeenCalled()
})
