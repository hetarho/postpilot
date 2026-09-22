import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

// AUTH-42: login and signup share one lockup and one field shape, so the word under the lockup
// and the sentence below the CTA are what tell them apart. The credential behaviour itself is
// pinned by app/routes/router.test.tsx.
describe('LoginPage', () => {
  afterEach(() => localStorage.clear())
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
  it('retains the saved id after a successful logout while the session ends', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/login')
    await user.type(await screen.findByLabelText('이메일 또는 아이디'), 'master')
    await user.type(screen.getByLabelText('비밀번호'), 'seed-only')
    await user.click(screen.getByRole('checkbox', { name: '아이디 저장' }))
    await user.click(screen.getByRole('checkbox', { name: '자동 로그인' }))
    await user.click(screen.getByRole('button', { name: '로그인' }))
    await user.click(await screen.findByRole('button', { name: '내 계정' }))
    await user.click(await screen.findByRole('button', { name: '로그아웃' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(await screen.findByLabelText('이메일 또는 아이디')).toHaveValue('master')
    expect(screen.getByLabelText('비밀번호')).toHaveValue('')
    expect(screen.getByRole('checkbox', { name: '아이디 저장' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: '자동 로그인' })).not.toBeChecked()
  })
})
