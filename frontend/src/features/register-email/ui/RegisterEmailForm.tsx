import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { AppFailureMessage, Button, FieldLabel, Notice, TextField } from '@/shared/ui'
import { useRegisterEmail } from '../api/useRegisterEmail'

export function RegisterEmailForm({ initialEmail = '' }: { initialEmail?: string }) {
  const { t } = useTranslation('auth')
  const register = useRegisterEmail()
  const [email, setEmail] = useState(initialEmail)
  const [sent, setSent] = useState(false)

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    register.mutate({ email }, { onSuccess: () => setSent(true) })
  }

  if (sent) {
    return (
      <Notice tone="success" role="status">
        {t('registerEmail.sent', { email })}
      </Notice>
    )
  }

  return (
    <form onSubmit={onSubmit} className="grid gap-3">
      <FieldLabel htmlFor="register-email">{t('field.email')}</FieldLabel>
      <TextField
        id="register-email"
        name="email"
        type="text"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        autoComplete="email"
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        required
      />
      {register.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={register.failure} />
        </Notice>
      )}
      <Button type="submit" variant="cta" pending={register.isPending}>
        {t('registerEmail.submit')}
      </Button>
    </form>
  )
}
