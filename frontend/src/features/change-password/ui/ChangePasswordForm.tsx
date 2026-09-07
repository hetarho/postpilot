import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { AppFailureMessage, Button, FieldLabel, Notice, TextField } from '@/shared/ui'
import { useChangePassword } from '../api/useChangePassword'

export function ChangePasswordForm({ onChanged }: { onChanged: () => void }) {
  const { t } = useTranslation('auth')
  const change = useChangePassword()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const wrongCurrent = change.failure?.reason === 'CURRENT_PASSWORD_WRONG'

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    change.mutate({ currentPassword, newPassword }, { onSuccess: onChanged })
  }

  return (
    <form onSubmit={onSubmit} className="grid gap-3">
      <div>
        <FieldLabel htmlFor="current-password">{t('changePassword.currentPassword')}</FieldLabel>
        <TextField
          id="current-password"
          type="password"
          value={currentPassword}
          onChange={(event) => setCurrentPassword(event.target.value)}
          autoComplete="current-password"
          required
          aria-invalid={wrongCurrent || undefined}
          className="mt-1.5"
        />
      </div>
      <div>
        <FieldLabel htmlFor="new-password">{t('changePassword.newPassword')}</FieldLabel>
        <TextField
          id="new-password"
          type="password"
          value={newPassword}
          onChange={(event) => setNewPassword(event.target.value)}
          autoComplete="new-password"
          minLength={8}
          maxLength={128}
          required
          className="mt-1.5"
        />
      </div>
      {change.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={change.failure} />
        </Notice>
      )}
      <Button type="submit" variant="cta" pending={change.isPending}>
        {t('changePassword.submit')}
      </Button>
    </form>
  )
}
