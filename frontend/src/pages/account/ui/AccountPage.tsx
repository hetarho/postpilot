import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { ChangePasswordForm } from '@/features/change-password'
import { RegisterEmailForm } from '@/features/register-email'
import { Badge, pageStyles, Typography } from '@/shared/ui'

export function AccountPage() {
  const { t } = useTranslation('auth')
  const { user } = useSession()
  const navigate = useNavigate()

  return (
    <main className={pageStyles()}>
      <Typography variant="display" as="h1">
        {t('accountSettings.heading')}
      </Typography>
      <section className="mt-6 grid gap-4" aria-labelledby="account-identity">
        <Typography variant="title" as="h2" id="account-identity">
          {t('accountSettings.identity')}
        </Typography>
        <div className="grid gap-1">
          <Typography variant="label">{t('accountSettings.id')}</Typography>
          <Typography variant="body" mono className="break-words">
            {user?.id}
          </Typography>
        </div>
        <div className="grid gap-1">
          <Typography variant="label">{t('field.email')}</Typography>
          <div className="flex flex-wrap items-center gap-2">
            <Typography variant="body" className="break-words">
              {user?.email || t('accountSettings.noEmail')}
            </Typography>
            {user?.email && (
              <Badge tone={user.emailVerified ? 'success' : 'neutral'}>
                {user.emailVerified
                  ? t('accountSettings.verified')
                  : t('accountSettings.unverified')}
              </Badge>
            )}
          </div>
        </div>
      </section>
      {!user?.emailVerified && (
        <section className="mt-8 grid gap-4" aria-labelledby="register-email-heading">
          <Typography variant="title" as="h2" id="register-email-heading">
            {t('registerEmail.heading')}
          </Typography>
          <Typography variant="body" className="text-content-secondary">
            {t('registerEmail.intro')}
          </Typography>
          <RegisterEmailForm initialEmail={user?.email} />
        </section>
      )}
      {user && (
        <section className="mt-8 grid gap-4" aria-labelledby="change-password-heading">
          <Typography variant="title" as="h2" id="change-password-heading">
            {t('changePassword.heading')}
          </Typography>
          {user.hasPassword ? (
            <>
              <Typography variant="body" className="text-content-secondary">
                {t('changePassword.intro')}
              </Typography>
              <ChangePasswordForm
                onChanged={() => {
                  void navigate({
                    to: '/login',
                    replace: true,
                    state: { notice: 'password-changed' },
                  })
                }}
              />
            </>
          ) : (
            <Typography variant="body" className="text-content-secondary">
              {t('changePassword.noPassword')}
            </Typography>
          )}
        </section>
      )}
    </main>
  )
}
