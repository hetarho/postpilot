import { expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import { renderAppAt } from '@/test/app'
it('redirects the authenticated library address to home without creating work', async () => {
  const calls: string[] = []
  const view = renderAppAt('/library', { user: { id: 'alice' }, calls })
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/'))
  const main = await screen.findByRole('main')
  expect(within(main).getAllByRole('link')).toHaveLength(3)
  expect(calls.filter((call) => /^(Create|Start|Generate|Save)/.test(call))).toEqual([])
})
