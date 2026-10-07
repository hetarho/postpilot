import { beforeEach, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  ClipGenerationService,
  DeleteClipProjectResponseSchema,
  type AppFailureReason,
} from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { DeleteClipProjectButton } from './DeleteClipProjectButton'
import { rememberClipEntry } from '@/entities/clip-project'

const navigate = vi.hoisted(() => vi.fn())
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => navigate }))
beforeEach(() => sessionStorage.clear())

function renderButton(
  options: {
    refusal?: AppFailureReason
    disabled?: boolean
    gate?: Promise<void>
    onDeleted?: () => void
    onReturn?: () => void | Promise<void>
  } = {},
) {
  const calls: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipGenerationService.method.deleteClipProject, async (req) => {
      calls.push(req.id)
      if (options.gate) await options.gate
      if (options.refusal) throw connectAppError(options.refusal, Code.FailedPrecondition)
      return create(DeleteClipProjectResponseSchema, {})
    })
  })
  const view = render(
    <DeleteClipProjectButton
      ownerId="alice"
      project={{ id: 'clip' }}
      disabled={options.disabled}
      onDeleted={options.onDeleted}
      onReturn={options.onReturn}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { ...view, calls, user: userEvent.setup() }
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

it('deletes once and uses its named history parent when there is no retained origin', async () => {
  navigate.mockClear()
  const { calls, user } = renderButton()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  await user.click(screen.getAllByRole('button', { name: '삭제', hidden: true })[1]!)
  expect(calls).toEqual(['clip'])
  expect(navigate).toHaveBeenCalledWith({ href: '/clips', replace: true })
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
it('returns a creation-origin clip to home only after server-confirmed deletion', async () => {
  navigate.mockClear()
  rememberClipEntry('alice', {
    path: '/',
    section: 'creation',
    filters: {},
    scrollY: 0,
    targetId: 'clip',
  })
  const { calls, user } = renderButton()
  await user.click(screen.getByRole('button', { name: '삭제' }))
  expect(navigate).not.toHaveBeenCalled()
  await user.click(screen.getAllByRole('button', { name: '삭제', hidden: true })[1]!)
  expect(calls).toEqual(['clip'])
  expect(navigate).toHaveBeenCalledWith({ href: '/', replace: true })
})
it('uses the injected return only after confirmed deletion and queue cleanup', async () => {
  navigate.mockClear()
  const order: string[] = []
  const { user } = renderButton({
    onDeleted: () => {
      order.push('cleanup')
    },
    onReturn: () => {
      order.push('return')
    },
  })
  await user.click(screen.getByRole('button', { name: '삭제' }))
  expect(order).toEqual([])
  await user.click(screen.getAllByRole('button', { name: '삭제', hidden: true })[1]!)
  await waitFor(() => expect(order).toEqual(['cleanup', 'return']))
  expect(navigate).not.toHaveBeenCalled()
})
it('a late confirmed deletion cleans the old queues but cannot navigate after its owner view has closed', async () => {
  navigate.mockClear()
  let release!: () => void
  const gate = new Promise<void>((done) => {
    release = done
  })
  const cleanup = vi.fn()
  const returned = vi.fn()
  const view = renderButton({ gate, onDeleted: cleanup, onReturn: returned })
  await view.user.click(screen.getByRole('button', { name: '삭제' }))
  await view.user.click(screen.getAllByRole('button', { name: '삭제', hidden: true })[1]!)
  await waitFor(() => expect(view.calls).toEqual(['clip']))
  view.unmount()
  await act(async () => {
    release()
    await gate
  })
  await waitFor(() => expect(cleanup).toHaveBeenCalledTimes(1))
  expect(returned).not.toHaveBeenCalled()
  expect(navigate).not.toHaveBeenCalled()
})
