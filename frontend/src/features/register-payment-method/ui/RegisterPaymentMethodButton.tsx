import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { TOSS_CLIENT_KEY } from '@/shared/config'
import { Button, FieldMessage, Typography, typographyStyles } from '@/shared/ui'
import { openTossBillingAuth } from '../lib/toss'

export function RegisterPaymentMethodButton({
  customerKey,
  registered = false,
  clientKey = TOSS_CLIENT_KEY,
  returnTo,
}: {
  customerKey: string
  registered?: boolean
  clientKey?: string
  returnTo?: string
}) {
  const { t } = useTranslation('billing')
  const { user } = useSession()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  if (!clientKey) return null

  const verified = Boolean(user?.emailVerified && user.email)
  const start = async () => {
    if (!verified || !user?.email) return
    setFailed(false)
    setPending(true)
    try {
      await openTossBillingAuth({ clientKey, customerKey, customerEmail: user.email, returnTo })
    } catch {
      setFailed(true)
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="grid w-fit gap-2">
      <Button
        variant="secondary"
        disabled={!verified}
        pending={pending}
        onClick={() => void start()}
      >
        {registered ? t('paymentMethod.change') : t('paymentMethod.register')}
      </Button>
      {!verified && (
        <Typography variant="meta" className="text-content-secondary max-w-measure">
          {t('paymentMethod.emailRequired')}{' '}
          <a
            href="/account"
            className={typographyStyles({
              variant: 'meta',
              className: 'text-link-fg underline',
            })}
          >
            {t('paymentMethod.verifyEmail')}
          </a>
        </Typography>
      )}
      {failed && <FieldMessage>{t('paymentMethod.openFailed')}</FieldMessage>}
    </div>
  )
}
