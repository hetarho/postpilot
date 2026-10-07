import { expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

it('offers compact chronological type views for ongoing work, failures and exports', async () => {
  renderAppAt('/library', { user: { id: 'alice' } })
  expect(await screen.findByRole('heading', { name: '작업 내역' })).toBeInTheDocument()
  const views = within(screen.getByRole('main')).getByRole('list')
  const links = within(views).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual(['/posts', '/clips'])
  expect(links[0]).toHaveTextContent('AI 결과와 글 내보내기')
  expect(links[1]).toHaveTextContent('실패한 시도와 영상 다운로드')
  expect(screen.queryByRole('link', { name: '새 글' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '새 클립' })).not.toBeInTheDocument()
})
