import type { ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { ChangePasswordForm } from '@/features/change-password'
import { RegisterEmailForm } from '@/features/register-email'
import {
  Badge,
  Button,
  Editable,
  ListGroup,
  TitleRow,
  Typography,
  listRowStyles,
  pageStyles,
} from '@/shared/ui'

/** 계정, opened from 설정 or the account menu: one group of rows that reads as the account's state.
 *  A row whose value can change carries a pencil that opens its form in place. */
export function AccountPage() {
  const { t } = useTranslation(['auth', 'common'])
  const { user } = useSession()
  const navigate = useNavigate()

  return (
    <main className={pageStyles({ width: 'prose', className: 'pt-4 sm:pt-10' })}>
      <TitleRow>
        <Typography variant="largeTitle">{t('accountSettings.heading')}</Typography>
      </TitleRow>

      {user && (
        <ListGroup className="mt-8">
          <li className={listRowStyles({ interactive: false })}>
            <AccountRow label={t('accountSettings.id')}>
              <Typography variant="label" mono className="break-words">
                {user.id}
              </Typography>
            </AccountRow>
          </li>
          <li className={listRowStyles({ interactive: false, className: 'py-2' })}>
            <Editable
              className="min-w-0 flex-1 items-center"
              editLabel={t('registerEmail.heading')}
              readOnly={user.emailVerified}
              edit={(exit) => (
                <RowForm label={t('field.email')} onCancel={exit}>
                  <RegisterEmailForm initialEmail={user.email} />
                </RowForm>
              )}
            >
              <AccountRow label={t('field.email')}>
                <span className="flex min-w-0 flex-wrap items-center justify-end gap-2">
                  <Typography variant="label" className="min-w-0 break-words">
                    {user.email || t('accountSettings.noEmail')}
                  </Typography>
                  {user.email && (
                    <Badge tone={user.emailVerified ? 'success' : 'neutral'}>
                      {user.emailVerified
                        ? t('accountSettings.verified')
                        : t('accountSettings.unverified')}
                    </Badge>
                  )}
                </span>
              </AccountRow>
            </Editable>
          </li>
          <li className={listRowStyles({ interactive: false, className: 'py-2' })}>
            {user.hasPassword ? (
              <Editable
                className="min-w-0 flex-1 items-center"
                editLabel={t('changePassword.heading')}
                edit={(exit) => (
                  <RowForm label={t('accountSettings.password')} onCancel={exit}>
                    <ChangePasswordForm
                      onChanged={() => {
                        void navigate({
                          to: '/login',
                          replace: true,
                          state: { notice: 'password-changed' },
                        })
                      }}
                    />
                  </RowForm>
                )}
              >
                <AccountRow label={t('accountSettings.password')}>
                  <Typography variant="label">{t('accountSettings.passwordSet')}</Typography>
                </AccountRow>
              </Editable>
            ) : (
              <div className="grid min-w-0 flex-1 gap-1">
                <span>{t('accountSettings.password')}</span>
                <Typography variant="label">{t('changePassword.noPassword')}</Typography>
              </div>
            )}
          </li>
        </ListGroup>
      )}
    </main>
  )
}

/** A row's name with its value at the far end, the value wrapping under the name when both do not fit. */
function AccountRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-x-3 gap-y-1">
      <span>{label}</span>
      {children}
    </div>
  )
}

/** A row opened in place: its name over the feature's own form, and the way back to the row. */
function RowForm({
  label,
  onCancel,
  children,
}: {
  label: string
  onCancel: () => void
  children: ReactNode
}) {
  const { t } = useTranslation('common')
  return (
    <div className="grid gap-3 py-2">
      <span>{label}</span>
      {children}
      <Button variant="ghost" onClick={onCancel} className="justify-self-start">
        {t('action.cancel')}
      </Button>
    </div>
  )
}
