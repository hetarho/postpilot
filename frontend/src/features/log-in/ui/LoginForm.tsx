import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLogin } from '@/entities/session'
import {
  AppFailureMessage,
  Button,
  Checkbox,
  FieldLabel,
  Logo,
  TextField,
  Typography,
} from '@/shared/ui'
import { readSavedLoginId, saveLoginId } from '../lib/saved-login-id'

type LoginFormProps = {
  rememberMe: boolean
  onRememberMeChange: (value: boolean) => void
  onSuccess: () => void
}

export function LoginForm({ rememberMe, onRememberMeChange, onSuccess }: LoginFormProps) {
  const { t } = useTranslation('auth')
  const login = useLogin()
  const [savedId] = useState(readSavedLoginId)
  const [loginId, setLoginId] = useState(savedId)
  const [password, setPassword] = useState('')
  const [rememberId, setRememberId] = useState(Boolean(savedId))
  const invalidCredentials = login.failure?.reason === 'INVALID_CREDENTIALS'

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (login.isPending) return
    login.mutate(
      { loginId, password, rememberMe },
      {
        onSuccess: () => {
          saveLoginId(rememberId ? loginId : '')
          onSuccess()
        },
      },
    )
  }

  return (
    <form onSubmit={onSubmit} className="w-full" aria-labelledby="login-heading">
      {/* The lockup is brand, not the heading: login and signup share it, and the word under
            it is what tells the two screens apart (AUTH-42). The compact phone lockup keeps the
            submit path above the software keyboard; wider screens give the mark its full presence. */}
      <div className="flex flex-col items-center gap-1 sm:gap-4">
        <img src="/favicon.svg" alt="" className="h-10 w-10 sm:h-20 sm:w-20" />
        <Logo className="h-8 sm:h-9" />
      </div>
      <Typography variant="display" as="h1" id="login-heading" className="mt-4 text-center sm:mt-6">
        {t('login.heading', { ns: 'auth' })}
      </Typography>
      <Typography variant="body" className="text-content-secondary mt-1 text-center">
        {t('login.intro', { ns: 'auth' })}
      </Typography>

      <FieldLabel htmlFor="loginId" className="mt-4 sm:mt-8">
        {t('login.id', { ns: 'auth' })}
      </FieldLabel>
      <TextField
        id="loginId"
        name="loginId"
        value={loginId}
        onChange={(event) => setLoginId(event.target.value)}
        disabled={login.isPending}
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
        aria-invalid={invalidCredentials || undefined}
        aria-describedby={invalidCredentials ? 'login-error' : undefined}
        className="mt-1.5"
      />

      <FieldLabel htmlFor="password" className="mt-4">
        {t('login.password', { ns: 'auth' })}
      </FieldLabel>
      <TextField
        id="password"
        name="password"
        type="password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        disabled={login.isPending}
        autoComplete="current-password"
        // The return key is a real submit path here, so it says 이동 rather than 줄바꿈.
        enterKeyHint="go"
        required
        aria-invalid={invalidCredentials || undefined}
        aria-describedby={invalidCredentials ? 'login-error' : undefined}
        className="mt-1.5"
      />

      <div className="mt-3 flex flex-wrap items-center justify-between gap-x-4">
        <FieldLabel className="flex min-h-11 cursor-pointer items-center gap-2">
          <Checkbox
            checked={rememberMe}
            disabled={login.isPending}
            onChange={(event) => onRememberMeChange(event.target.checked)}
          />
          {t('loginPreferences.rememberMe')}
        </FieldLabel>
        <FieldLabel className="flex min-h-11 cursor-pointer items-center gap-2">
          <Checkbox
            checked={rememberId}
            disabled={login.isPending}
            onChange={(event) => {
              setRememberId(event.target.checked)
              if (!event.target.checked) saveLoginId('')
            }}
          />
          {t('loginPreferences.rememberId')}
        </FieldLabel>
      </div>

      {/* Under the fields it describes and ABOVE the button, not after it (design-language §7,
            §4.3). Below the button it was both the lowest thing on the keyboard-covered screen and
            a ~32px insertion that shifted the whole form the instant the thumb lifted off 로그인. */}
      {login.failure && (
        <Typography
          variant="body"
          as="div"
          id="login-error"
          role="alert"
          className="text-field-error mt-3 break-words"
        >
          <AppFailureMessage failure={login.failure} />
        </Typography>
      )}

      {/* `pending`, not a label swap: 로그인 → 확인 중… resizes the target under the thumb that
            just pressed it (§6). */}
      <Button type="submit" variant="cta" pending={login.isPending} className="mt-4 w-full sm:mt-6">
        {t('login.submit', { ns: 'auth' })}
      </Button>
    </form>
  )
}
