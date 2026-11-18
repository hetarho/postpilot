import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { renderAppAt } from '@/test/app'

// 기억's screen code, held back the way a first visit's chunk is still on its way.
const chunk = vi.hoisted(() => {
  let release = () => {}
  const arrived = new Promise<void>((resolve) => {
    release = resolve
  })
  return { arrived, release: () => release() }
})

vi.mock('@/pages/memories', async (importOriginal) => {
  await chunk.arrived
  return importOriginal()
})

/** A first visit waits on its screen's code while the router already holds the new address. The
 *  shell read that address, so it drew the next screen's way back on a row of its own over the
 *  screen still showing — pushing it down — and took it away again when the screen arrived with
 *  its own ← in its title row. The shell now follows the screen the outlet is drawing. */
it('keeps the shell on the screen it is drawing until the next screen arrives', async () => {
  const user = userEvent.setup()
  const view = renderAppAt('/', { user: { id: 'alice' } })
  await screen.findByRole('list', { name: '글쓰기 준비' })
  const home = screen.getByRole('main')

  await user.click(screen.getByRole('link', { name: '기억 화면으로 가기' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/memories'))
  expect(screen.getByRole('main')).toBe(home)
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()

  await act(async () => chunk.release())
  await screen.findByRole('heading', { level: 1, name: '기억' })
  expect(within(screen.getByRole('main')).getByRole('link', { name: '홈' })).toHaveAttribute(
    'href',
    '/',
  )
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()
})
