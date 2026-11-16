import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, it, vi } from 'vitest'
import { ConfirmedAction } from './ConfirmedAction'

function Harness({ run, onDone }: { run: () => Promise<unknown>; onDone: () => void }) {
  const [failed, setFailed] = useState(false)
  return (
    <ConfirmedAction
      trigger={(open) => <button onClick={open}>삭제</button>}
      failed={failed}
      failure={<p>실패했습니다</p>}
      title="삭제할까요"
      confirmLabel="확인"
      pending={false}
      run={() => run().catch((error: unknown) => (setFailed(true), Promise.reject(error)))}
      onDone={onDone}
    >
      설명
    </ConfirmedAction>
  )
}

it('opens the dialog from the trigger', async () => {
  render(<Harness run={vi.fn()} onDone={vi.fn()} />)
  expect(screen.queryByRole('dialog')).toBeNull()
  await userEvent.setup().click(screen.getByRole('button', { name: '삭제' }))
  expect(screen.getByRole('dialog')).toBeInTheDocument()
})

it('closes and calls onDone once after a successful run', async () => {
  const run = vi.fn().mockResolvedValue(undefined)
  const onDone = vi.fn()
  render(<Harness run={run} onDone={onDone} />)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.click(screen.getByRole('button', { name: '확인' }))
  await waitFor(() => expect(onDone).toHaveBeenCalledOnce())
  expect(run).toHaveBeenCalledOnce()
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

it('closes and shows the failure without onDone when the run fails', async () => {
  const onDone = vi.fn()
  render(<Harness run={vi.fn().mockRejectedValue(new Error('no'))} onDone={onDone} />)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.click(screen.getByRole('button', { name: '확인' }))
  expect(await screen.findByText('실패했습니다')).toBeInTheDocument()
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(onDone).not.toHaveBeenCalled()
})

it('cancel closes without running', async () => {
  const run = vi.fn()
  render(<Harness run={run} onDone={vi.fn()} />)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.keyboard('{Escape}')
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(run).not.toHaveBeenCalled()
})
