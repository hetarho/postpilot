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
    // Anchored to the TOP, never centred. The software keyboard covers the bottom ~40% of the
    // screen and does not resize the layout viewport, so centring parked 로그인 and the failure
    // message behind it on 360, 390 and 430 alike, with no scroll overflow to lift them out of it
    // (design-language §8.3). `pt-12` rather than a larger inset because the block is 344px once
    // the failure message is in it: on the shortest phone (360x640, keyboard line ~384) that is
    // what keeps the message itself fully in view. `min-h-full` stays — it is what paints the
    // page's plane down the whole screen.
    <main className="bg-surface-base text-content-primary flex min-h-full items-start justify-center px-4 pt-12 pb-10 sm:px-6">
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
          // iOS defaults an <input type="text"> to `autocapitalize="sentences"`, so `hrlee`
          // is submitted as `Hrlee` and the server answers with the one generic failure it is
          // required to give — a silent loop the user cannot explain (design-language §7).
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          enterKeyHint="next"
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
          // The return key is a real submit path here, so it says 이동 rather than 줄바꿈.
          enterKeyHint="go"
          required
          aria-invalid={login.isError || undefined}
          aria-describedby={login.isError ? 'login-error' : undefined}
          className="mt-1.5"
        />

        {/* Under the fields it describes and ABOVE the button, not after it (design-language §7,
            §4.3). Below the button it was both the lowest thing on the keyboard-covered screen and
            a ~32px insertion that shifted the whole form the instant the thumb lifted off 로그인. */}
        {login.isError && (
          <FieldMessage id="login-error" className="mt-3">
            {LOGIN_FAILED}
          </FieldMessage>
        )}

        {/* `pending`, not a label swap: 로그인 → 확인 중… resizes the target under the thumb that
            just pressed it (§6). */}
        <Button type="submit" variant="cta" pending={login.isPending} className="mt-6 w-full">
          로그인
        </Button>
      </form>
    </main>
  )
}
