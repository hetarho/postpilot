import { useState, type FormEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useLogin } from '@/entities/session'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import { Button, FieldLabel, FieldMessage, TextField } from '@/shared/ui'

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
    <main className="bg-surface-base text-content-primary flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <form onSubmit={onSubmit} className="w-full max-w-xs">
        <h1 className="text-2xl font-semibold tracking-tight">Postpilot</h1>
        <p className="text-content-secondary mt-1 text-sm">계속하려면 로그인하세요</p>

        <FieldLabel htmlFor="loginId" className="mt-8">
          아이디
        </FieldLabel>
        <TextField
          id="loginId"
          name="loginId"
          value={loginId}
          onChange={(event) => setLoginId(event.target.value)}
          autoComplete="username"
          autoFocus
          required
          aria-invalid={login.isError || undefined}
          aria-describedby={login.isError ? 'login-error' : undefined}
          className="mt-1.5"
        />

        <FieldLabel htmlFor="password" className="mt-4">
          비밀번호
        </FieldLabel>
        <TextField
          id="password"
          name="password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          autoComplete="current-password"
          required
          aria-invalid={login.isError || undefined}
          aria-describedby={login.isError ? 'login-error' : undefined}
          className="mt-1.5"
        />

        <Button type="submit" variant="cta" disabled={login.isPending} className="mt-6 w-full">
          {login.isPending ? '확인 중…' : '로그인'}
        </Button>

        {login.isError && (
          <FieldMessage id="login-error" className="mt-3">
            {LOGIN_FAILED}
          </FieldMessage>
        )}
      </form>
    </main>
  )
}
