import { Code, ConnectError, type Transport } from '@connectrpc/connect'
import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport } from '@/test/session'

const ROWS = ['First', 'Middle', 'Last'].map((name) => ({
  id: name.toLowerCase(),
  name,
  compositionBody: '<clip version="1"/>',
  projectCount: name === 'Middle' ? 2 : 0,
}))

function mount(path: string, { refuseDelete = false } = {}) {
  const calls: string[] = []
  const base = createFakeAuthTransport({
    user: { id: 'alice' },
    existingSetup: true,
    clips: { templates: ROWS },
  })
  const transport: Transport = {
    ...base,
    unary: async (...args) => {
      const method = args[0].name
      calls.push(method)
      if (method === 'DeleteVideoTemplate' && refuseDelete)
        throw new ConnectError('refused', Code.PermissionDenied)
      return base.unary(...args)
    },
  }
  return { ...renderAppAt(path, { transport }), calls }
}

async function openDelete(user: ReturnType<typeof userEvent.setup>, name: string) {
  const button = await screen.findByRole('button', { name: `${name} 삭제` })
  button.focus()
  await user.keyboard('{Enter}')
  return within(await screen.findByRole('dialog'))
}

// Deleting lives on the template's own reading, never on a directory tile (CLIP-42): the reading
// warns how many clips lose it, deletes on confirmation and returns to the list in place of itself.
describe('deleting a video template from its reading', () => {
  it('confirms with the clip count, then returns to the directory without the template', async () => {
    const user = userEvent.setup()
    const { router, calls } = mount('/video-templates/middle')
    const dialog = await openDelete(user, 'Middle')
    expect(dialog.getByText(/클립 2개/)).toBeInTheDocument()
    dialog.getByRole('button', { name: '삭제' }).focus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(router.state.location.pathname).toBe('/video-templates'))
    // `replace`: Back does not return to the reading of a template that no longer exists.
    expect(router.history.length).toBe(1)
    await screen.findByRole('link', { name: /^First/ })
    expect(screen.queryByRole('link', { name: /^Middle/ })).toBeNull()
    // No render, AI or authoring work starts on the way (CLIP-208).
    expect(calls.filter((call) => /Start|Generate|Render|Request/.test(call))).toEqual([])
  })

  it('stays on the reading with focus at the opener after cancellation', async () => {
    const user = userEvent.setup()
    const { router } = mount('/video-templates/middle')
    const dialog = await openDelete(user, 'Middle')
    dialog.getByRole('button', { name: '취소' }).focus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Middle 삭제' })).toHaveFocus()
    expect(router.state.location.pathname).toBe('/video-templates/middle')
  })

  it('keeps the reading after a refused delete', async () => {
    const user = userEvent.setup()
    const { router } = mount('/video-templates/middle', { refuseDelete: true })
    const dialog = await openDelete(user, 'Middle')
    dialog.getByRole('button', { name: '삭제' }).focus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(router.state.location.pathname).toBe('/video-templates/middle')
    expect(screen.getByRole('button', { name: 'Middle 삭제' })).toBeInTheDocument()
  })

  it('offers no delete on any directory tile', async () => {
    mount('/video-templates')
    const directory = within(await screen.findByRole('region', { name: '저장된 영상 템플릿' }))
    expect(directory.getAllByRole('listitem')).toHaveLength(3)
    expect(directory.queryByRole('button')).toBeNull()
  })
})
