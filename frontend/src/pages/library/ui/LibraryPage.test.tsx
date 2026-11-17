import { expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import { renderAppAt } from '@/test/app'
// The retired library address was the account's work history; the 글 tab is that place now, so
// the old address lands there — and arriving creates no work.
it('redirects the authenticated library address to the 글 tab without creating work', async () => {
  const calls: string[] = []
  const view = renderAppAt('/library', { user: { id: 'alice' }, calls })
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/posts'))
  expect(view.router.state.location.href).toBe('/posts')
  const main = await screen.findByRole('main')
  expect(
    await within(main).findByRole('heading', { level: 1, name: '네이버 블로그' }),
  ).toBeInTheDocument()
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '글' }),
  ).toHaveAttribute('aria-current', 'page')
  expect(calls.filter((call) => /^(Create|Start|Generate|Save)/.test(call))).toEqual([])
})
