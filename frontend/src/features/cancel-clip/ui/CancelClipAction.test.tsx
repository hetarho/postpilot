import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import type { ClipAccounting } from '@/entities/clip-project'
import { CancelClipAction } from './CancelClipAction'

const job = (patch: Partial<GenerationJob> = {}): GenerationJob => ({
  id: 'job',
  kind: 'generate_clip',
  status: 'running',
  stage: 'prepare',
  progressDone: 0,
  progressTotal: 1,
  postSlug: '',
  failure: undefined,
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '',
  updatedAt: '',
  targetLanguage: undefined,
  canCancel: true,
  cancellationPolicyVersion: 1,
  ...patch,
})
const action = () => ({
  cancel: vi.fn(async () => {}),
  checkAgain: vi.fn(async () => {}),
  pending: false,
  uncertain: false,
  cancelling: false,
  failure: undefined,
})

it('dismisses with Escape without cancelling and returns focus to the trigger', async () => {
  const a = action()
  render(<CancelClipAction action={a} job={job()} />)
  const trigger = screen.getByRole('button', { name: '취소' })
  await userEvent.click(trigger)
  expect(screen.getByRole('dialog')).toBeVisible()
  await userEvent.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
  expect(a.cancel).not.toHaveBeenCalled()
})

it.each(['completed', 'replaced', 'accepted'] as const)(
  'invalidates confirmation when the job is %s',
  async (change) => {
    const a = action()
    const view = render(<CancelClipAction action={a} job={job()} />)
    await userEvent.click(screen.getByRole('button', { name: '취소' }))
    expect(screen.getByRole('dialog')).toBeVisible()
    view.rerender(
      <CancelClipAction
        action={a}
        job={job(
          change === 'completed'
            ? { status: 'done' }
            : change === 'replaced'
              ? { id: 'new-job' }
              : { cancelRequestedAt: '2026-09-13T00:00:00Z' },
        )}
      />,
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(a.cancel).not.toHaveBeenCalled()
    if (change === 'replaced') {
      await userEvent.click(screen.getByRole('button', { name: '취소' }))
      await userEvent.click(
        within(screen.getByRole('dialog')).getByRole('button', { name: '제작 취소' }),
      )
      expect(a.cancel).toHaveBeenCalledTimes(1)
    }
  },
)

it.each([
  ['manual', 'not_reserved', /다시 렌더는 취소해도 크레딧이 차감되지/],
  ['ai', 'not_reserved', /아직 크레딧을 예약하지 않아/],
  ['ai', 'exempt', /실제 크레딧 차감이 없어요/],
  ['ai', 'reserved', /남은 예약 크레딧 50%/],
] as const)(
  'shows the applicable %s/%s cost in the guard and confirms only once',
  async (kind, status, message) => {
    const a = action()
    const accounting: ClipAccounting = {
      jobId: 'job',
      status,
      settled: false,
      reservedCredits: status === 'reserved' ? 60 : 0,
    }
    render(
      <CancelClipAction
        action={a}
        job={job({ kind: kind === 'manual' ? 'render_clip' : 'generate_clip' })}
        accounting={accounting}
      />,
    )
    await userEvent.dblClick(screen.getByRole('button', { name: '취소' }))
    const dialog = within(screen.getByRole('dialog'))
    expect(dialog.getByText(message)).toBeVisible()
    if (status === 'reserved') expect(dialog.getByText(/60크레딧/)).toBeVisible()
    expect(a.cancel).not.toHaveBeenCalled()
    await userEvent.dblClick(dialog.getByRole('button', { name: '제작 취소' }))
    expect(a.cancel).toHaveBeenCalledTimes(1)
  },
)
