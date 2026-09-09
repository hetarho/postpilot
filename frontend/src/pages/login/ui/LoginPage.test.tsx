import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

// AUTH-42: login and signup share one lockup and one field shape, so the word under the lockup
// and the sentence below the CTA are what tell them apart. The credential behaviour itself is
// pinned by app/routes/router.test.tsx.
describe('LoginPage', () => {
  it('is titled 로그인 and leads to signup through a question with its answer', async () => {
    renderAppAt('/login?redirect=%2Fposts')

    expect(await screen.findByRole('heading', { level: 1, name: '로그인' })).toBeInTheDocument()
    expect(screen.getByText('계정이 없으세요?')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '회원가입' })).toHaveAttribute(
      'href',
      '/signup?redirect=%2Fposts',
    )
  })

  it('keeps the forgot-password and About links in the row below the sentence', async () => {
    renderAppAt('/login')

    expect(await screen.findByRole('link', { name: '비밀번호 찾기' })).toHaveAttribute(
      'href',
      '/forgot-password',
    )
    expect(screen.getByRole('link', { name: 'Postpilot이란?' })).toHaveAttribute('href', '/about')
    expect(screen.getByRole('navigation', { name: '계정 메뉴' })).toBeInTheDocument()
    // The two-field shape is login's; the confirmation field belongs to signup alone.
    expect(screen.queryByLabelText('비밀번호 확인')).not.toBeInTheDocument()
  })
})
