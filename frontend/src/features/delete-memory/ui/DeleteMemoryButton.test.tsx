import { expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { DeleteMemoryResponseSchema, MemoryService } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { DeleteMemoryButton } from './DeleteMemoryButton'

function renderButton(options: { refused?: boolean } = {}) {
  const deletions: string[] = []
  const onDeleted = vi.fn()
  const transport = createRouterTransport(({ rpc }) => {
    rpc(MemoryService.method.deleteMemory, (req) => {
      deletions.push(req.id)
      if (options.refused) throw connectAppError('MEMORY_NOT_FOUND', Code.NotFound)
      return create(DeleteMemoryResponseSchema, {})
    })
  })
  render(<DeleteMemoryButton ownerId="alice" memoryId="memory-place" onDeleted={onDeleted} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return { deletions, onDeleted, user: userEvent.setup() }
}

async function openAndConfirm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: '이 기억 삭제' }))
  await user.click(screen.getByRole('button', { name: '삭제', hidden: true }))
}

it('tells the page once the server has deleted the memory', async () => {
  const { deletions, onDeleted, user } = renderButton()
  await openAndConfirm(user)

  await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1))
  expect(deletions).toEqual(['memory-place'])
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

it('keeps the refusal beside the button and tells the page nothing when the delete fails', async () => {
  const { deletions, onDeleted, user } = renderButton({ refused: true })
  await openAndConfirm(user)

  expect(await screen.findByRole('alert')).not.toBeEmptyDOMElement()
  expect(deletions).toEqual(['memory-place'])
  await waitFor(() =>
    expect(screen.queryByRole('dialog', { hidden: true })).not.toBeInTheDocument(),
  )
  expect(onDeleted).not.toHaveBeenCalled()
})
