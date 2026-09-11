import { expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { ClipService, DeleteClipProjectResponseSchema, type AppFailureReason } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { DeleteClipProjectButton } from './DeleteClipProjectButton'

const navigate = vi.hoisted(() => vi.fn())
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => navigate }))

function renderButton(options: { refusal?: AppFailureReason; disabled?: boolean } = {}) {
  const calls: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipService.method.deleteClipProject, (req) => {
      calls.push(req.id)
      if (options.refusal) throw connectAppError(options.refusal, Code.FailedPrecondition)
      return create(DeleteClipProjectResponseSchema, {})
    })
  })
  render(
    <DeleteClipProjectButton
      ownerId="alice"
      project={{ id: 'clip' }}
      disabled={options.disabled}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { calls, user: userEvent.setup() }
}

it('destroys nothing until the confirmation is accepted', async () => {
  navigate.mockClear()
  const { calls, user } = renderButton()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  // The sheet says what goes with the project (CLIP-24), and dismissing it deletes nothing.
  expect(await screen.findByText(/분석과 편집 내용, 생성된 영상이 함께 삭제돼요/)).toBeVisible()
  await user.click(screen.getByRole('button', { name: '취소', hidden: true }))
  expect(calls).toEqual([])
  expect(navigate).not.toHaveBeenCalled()
})

it('deletes once and leaves for the directory', async () => {
  navigate.mockClear()
  const { calls, user } = renderButton()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.click(screen.getAllByRole('button', { name: '삭제', hidden: true })[1]!)
  expect(calls).toEqual(['clip'])
  expect(navigate).toHaveBeenCalledWith({ to: '/clips', replace: true })
})

it('keeps the owner on the project and reports a refusal beside the trigger', async () => {
  navigate.mockClear()
  const { user } = renderButton({ refusal: 'CLIP_BUSY' })
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.click(screen.getAllByRole('button', { name: '삭제', hidden: true })[1]!)
  // The refusal takes its own line on the top row rather than staying behind the scrim.
  expect(await screen.findByRole('alert')).toBeVisible()
  expect(navigate).not.toHaveBeenCalled()
})

it('is refused outright while the workspace is busy', () => {
  renderButton({ disabled: true })
  expect(screen.getByRole('button', { name: '삭제' })).toBeDisabled()
})
