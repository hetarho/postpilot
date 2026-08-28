import { useState, type FormEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useLogin } from '@/entities/session'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'

/** One message for every failure. The server already refuses to distinguish an unknown
 *  id from a wrong password (spec/policy/auth.md); showing anything more specific here
 *  would hand back the id-enumeration signal the backend works to hide. */
const LOGIN_FAILED = '아이디 또는 비밀번호가 맞지 않아요'

export function LoginPage() {
  const { redirect } = useSearch({ from: '/login' })
  const navigate = useNavigate()
  const login = useLogin()
  const [loginId, setLoginId] = useState('')
  const [password, setPassword] = useState('')

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    login.mutate(
      { loginId, password },
      {
        onSuccess: () => {
          // The guard put the blocked destination in `redirect`; anything else is not
          // ours to follow.
          void navigate({ to: isInAppPath(redirect) ? redirect : SIGNED_IN_HOME, replace: true })
        },
      },
    )
  }

  return (
    <main className="flex min-h-full items-center justify-center bg-bg px-6 text-text">
      <form onSubmit={onSubmit} className="w-full max-w-xs">
        <h1 className="text-2xl font-semibold tracking-tight">Postpilot</h1>
        <p className="mt-1 text-sm text-text-muted">계속하려면 로그인하세요</p>

        <label htmlFor="loginId" className="mt-8 block text-sm text-text-muted">
          아이디
        </label>
        <input
          id="loginId"
          name="loginId"
          value={loginId}
          onChange={(event) => setLoginId(event.target.value)}
          autoComplete="username"
          autoFocus
          required
          className="mt-1.5 w-full rounded-md bg-surface px-3 py-2 text-sm placeholder:text-text-faint focus:bg-surface-raised"
        />

        <label htmlFor="password" className="mt-4 block text-sm text-text-muted">
          비밀번호
        </label>
        <input
          id="password"
          name="password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          autoComplete="current-password"
          required
          className="mt-1.5 w-full rounded-md bg-surface px-3 py-2 text-sm placeholder:text-text-faint focus:bg-surface-raised"
        />

        <button
          type="submit"
          disabled={login.isPending}
          className="mt-6 w-full rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:bg-primary-hover disabled:opacity-50"
        >
          {login.isPending ? '확인 중…' : '로그인'}
        </button>

        {login.isError && (
          <p role="alert" className="mt-3 text-sm text-danger">
            {LOGIN_FAILED}
          </p>
        )}
      </form>
    </main>
  )
}
