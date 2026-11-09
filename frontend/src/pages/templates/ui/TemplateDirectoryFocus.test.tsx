import { Code, ConnectError, type Transport } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport } from '@/test/session'

const ROWS = ['First', 'Middle', 'Last'].map((name) => ({
  id: name.toLowerCase(),
  name,
  body: '<write>text</write>',
}))

function mount({ rows = ROWS, holdReread = false, refuseDelete = false } = {}) {
  let deleting = false
  let releaseReread = () => {}
  const reread = new Promise<void>((resolve) => {
    releaseReread = resolve
  })
  const base = createFakeAuthTransport({
    user: { id: 'alice' },
    existingSetup: true,
    templates: { templates: rows },
  })
  const transport: Transport = {
    ...base,
    unary: async (...args) => {
      const method = args[0].name
      if (method === 'DeleteTemplate') {
        if (refuseDelete) throw new ConnectError('refused', Code.PermissionDenied)
        deleting = true
      }
      const response = await base.unary(...args)
      if (method === 'ListTemplates' && deleting && holdReread) await reread
      return response
    },
  }
  return { ...renderAppAt('/templates', { transport }), releaseReread }
}

async function openDelete(user: ReturnType<typeof userEvent.setup>, name: string) {
  const button = await screen.findByRole('button', { name: `${name} 삭제` })
  button.focus()
  await user.keyboard('{Enter}')
  return within(await screen.findByRole('dialog'))
}

async function confirmDelete(user: ReturnType<typeof userEvent.setup>, name: string) {
  const dialog = await openDelete(user, name)
  dialog.getByRole('button', { name: '삭제' }).focus()
  await user.keyboard('{Enter}')
}

describe('post template keyboard focus after deletion', () => {
  it.each([
    { name: 'Middle', rows: ROWS, next: 'Last' },
    { name: 'Last', rows: ROWS, next: 'Middle' },
    { name: 'First', rows: [ROWS[0]], next: null },
  ])('keeps the keyboard position after deleting $name', async ({ name, rows, next }) => {
    const user = userEvent.setup()
    mount({ rows })
    await confirmDelete(user, name)
    await waitFor(() => expect(screen.queryByRole('button', { name: `${name} 삭제` })).toBeNull())
    const target = next
      ? screen.getByRole('button', { name: `${next} 삭제` })
      : screen.getByRole('link', { name: '새 템플릿' })
    expect(target).toHaveFocus()
  })

  it('leaves focus at the opener after cancellation and an unrelated reread', async () => {
    const user = userEvent.setup()
    const { queryClient } = mount()
    const dialog = await openDelete(user, 'Middle')
    dialog.getByRole('button', { name: '취소' }).focus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Middle 삭제' })).toHaveFocus()
    await act(() => queryClient.invalidateQueries())
    expect(screen.getByRole('button', { name: 'Middle 삭제' })).toHaveFocus()
  })

  it('leaves a refused delete focused at its opener, without a later stale request', async () => {
    const user = userEvent.setup()
    const { queryClient } = mount({ refuseDelete: true })
    await confirmDelete(user, 'Middle')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Middle 삭제' })).toHaveFocus()
    const elsewhere = screen.getByRole('button', { name: 'First 삭제' })
    elsewhere.focus()
    await act(() => queryClient.invalidateQueries())
    expect(elsewhere).toHaveFocus()
  })

  it('waits for the delayed reread before recovering the removed row focus', async () => {
    const user = userEvent.setup()
    const { releaseReread } = mount({ holdReread: true })
    await confirmDelete(user, 'Middle')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Middle 삭제' })).toHaveFocus()
    await act(async () => releaseReread())
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Middle 삭제' })).toBeNull())
    expect(screen.getByRole('button', { name: 'Last 삭제' })).toHaveFocus()
  })

  it('preserves focus moved elsewhere during a delayed reread and settles once', async () => {
    const user = userEvent.setup()
    const { queryClient, releaseReread } = mount({ holdReread: true })
    await confirmDelete(user, 'Middle')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    const elsewhere = screen.getByRole('button', { name: 'First 삭제' })
    elsewhere.focus()
    await act(async () => releaseReread())
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Middle 삭제' })).toBeNull())
    expect(elsewhere).toHaveFocus()
    elsewhere.blur()
    await act(() => queryClient.invalidateQueries())
    expect(document.body).toHaveFocus()
  })
})
