import { expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { DeletePostResponseSchema, PostService, type AppFailureReason } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { DeletePostButton } from './DeletePostButton'

const navigate = vi.hoisted(() => vi.fn())
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => navigate }))

const POST = { slug: 'post-a', title: '제주 3일' }

function renderButton(options: { refusal?: AppFailureReason; onDeleted?: () => void } = {}) {
  const calls: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.deletePost, (req) => {
      calls.push(req.slug)
      if (options.refusal) {
        throw connectAppError(options.refusal, Code.FailedPrecondition)
      }
      return create(DeletePostResponseSchema, {})
    })
  })
  render(<DeletePostButton post={POST} onDeleted={options.onDeleted} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return { calls, user: userEvent.setup() }
}

async function openAndConfirm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '글 삭제하기' }))
  await user.click(screen.getByRole('button', { name: '삭제', hidden: true }))
}

it('deletes nothing when the confirmation is dismissed', async () => {
  navigate.mockClear()
  const { calls, user } = renderButton()

  await user.click(screen.getByRole('button', { name: '글 삭제하기' }))
  expect(screen.getByText(/영구히 지워져요/)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '취소', hidden: true }))

  expect(calls).toEqual([])
  expect(navigate).not.toHaveBeenCalled()
})

it("deletes, ends the post's autosave, then leaves for the list", async () => {
  navigate.mockClear()
  const onDeleted = vi.fn()
  const { calls, user } = renderButton({ onDeleted })

  await openAndConfirm(user)

  expect(calls).toEqual(['post-a'])
  // The queues have to stop BEFORE the navigation unmounts the editor, or a retry outlives
  // the post it belongs to.
  expect(onDeleted).toHaveBeenCalledBefore(navigate)
  expect(navigate).toHaveBeenCalledWith({ to: '/posts' })
})

it('explains a live publication and stays on the editor', async () => {
  navigate.mockClear()
  const { user } = renderButton({ refusal: 'POST_PUBLISHING' })

  await openAndConfirm(user)

  expect(await screen.findByRole('alert')).toHaveTextContent('발행을 취소하거나 끝낸 뒤에')
  expect(navigate).not.toHaveBeenCalled()
})

it('distinguishes a running AI job from a live publication', async () => {
  navigate.mockClear()
  const { user } = renderButton({ refusal: 'POST_BUSY' })

  await openAndConfirm(user)

  expect(await screen.findByRole('alert')).toHaveTextContent('AI 작업이 진행 중이에요')
  expect(navigate).not.toHaveBeenCalled()
})
