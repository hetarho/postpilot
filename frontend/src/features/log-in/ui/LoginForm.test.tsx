import { useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { SAVED_LOGIN_ID_KEY } from '../config/storage'
import { LoginForm } from './LoginForm'

function renderForm(loginFails = false) {
  const onSuccess = vi.fn()
  const onLogin = vi.fn()
  const transport = createFakeAuthTransport({ loginFails, onLogin })
  function Form() {
    const [rememberMe, setRememberMe] = useState(false)
    return (
      <LoginForm rememberMe={rememberMe} onRememberMeChange={setRememberMe} onSuccess={onSuccess} />
    )
  }
  return {
    ...render(<Form />, { wrapper: withProviders(transport, createTestQueryClient()) }),
    onSuccess,
    onLogin,
  }
}

async function enterCredentials(id = 'master') {
  const user = userEvent.setup()
  const field = screen.getByLabelText('이메일 또는 아이디')
  await user.clear(field)
  await user.type(field, id)
  await user.type(screen.getByLabelText('비밀번호'), 'seed-only')
  return user
}

describe('LoginForm preferences', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('starts with both options off and no stored credentials', () => {
    renderForm()
    expect(screen.getByRole('checkbox', { name: '자동 로그인' })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: '아이디 저장' })).not.toBeChecked()
    expect(screen.getByLabelText('이메일 또는 아이디')).toHaveValue('')
    expect(screen.getByLabelText('비밀번호')).toHaveValue('')
  })

  it.each([
    [false, false],
    [true, false],
    [false, true],
    [true, true],
  ])('submits automatic login=%s independently of saving id=%s', async (rememberMe, rememberId) => {
    const view = renderForm()
    const user = await enterCredentials()
    if (rememberMe) await user.click(screen.getByRole('checkbox', { name: '자동 로그인' }))
    if (rememberId) await user.click(screen.getByRole('checkbox', { name: '아이디 저장' }))
    expect(localStorage.getItem(SAVED_LOGIN_ID_KEY)).toBeNull()
    await user.click(screen.getByRole('button', { name: '로그인' }))
    await waitFor(() => expect(view.onSuccess).toHaveBeenCalledOnce())
    expect(view.onLogin).toHaveBeenCalledWith(
      expect.objectContaining({ loginId: 'master', password: 'seed-only', rememberMe }),
    )
    expect(localStorage.getItem(SAVED_LOGIN_ID_KEY)).toBe(rememberId ? 'master' : null)
    expect(localStorage.length).toBe(rememberId ? 1 : 0)
    view.unmount()
    renderForm()
    expect(screen.getByLabelText('이메일 또는 아이디')).toHaveValue(rememberId ? 'master' : '')
    expect(screen.getByLabelText('비밀번호')).toHaveValue('')
    expect(screen.getByRole('checkbox', { name: '아이디 저장' })).toHaveProperty(
      'checked',
      rememberId,
    )
    expect(screen.getByRole('checkbox', { name: '자동 로그인' })).not.toBeChecked()
  })

  it('removes a saved id immediately when unchecked, before submitting', async () => {
    localStorage.setItem(SAVED_LOGIN_ID_KEY, 'base')
    const view = renderForm()
    await userEvent.click(screen.getByRole('checkbox', { name: '아이디 저장' }))
    expect(localStorage.getItem(SAVED_LOGIN_ID_KEY)).toBeNull()
    expect(screen.getByLabelText('이메일 또는 아이디')).toHaveValue('base')
    view.unmount()
    renderForm()
    expect(screen.getByLabelText('이메일 또는 아이디')).toHaveValue('')
  })

  it('keeps the previously saved id after a failed login', async () => {
    localStorage.setItem(SAVED_LOGIN_ID_KEY, 'base')
    const view = renderForm(true)
    const user = await enterCredentials('master')
    await user.click(screen.getByRole('button', { name: '로그인' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(localStorage.getItem(SAVED_LOGIN_ID_KEY)).toBe('base')
    expect(view.onSuccess).not.toHaveBeenCalled()
  })

  it('still signs in when reading, saving and removing local storage throw', async () => {
    for (const method of ['getItem', 'setItem', 'removeItem'] as const) {
      vi.spyOn(window.localStorage, method).mockImplementation(() => {
        throw new Error('storage blocked')
      })
    }
    const view = renderForm()
    const user = await enterCredentials()
    const save = screen.getByRole('checkbox', { name: '아이디 저장' })
    await user.click(save)
    await user.click(save)
    await user.click(save)
    await user.click(screen.getByRole('button', { name: '로그인' }))
    await waitFor(() => expect(view.onSuccess).toHaveBeenCalledOnce())
  })
})
