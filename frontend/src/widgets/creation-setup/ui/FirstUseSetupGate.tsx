import { useEffect, type ReactNode } from 'react'
import { Navigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSetupAvailability } from '@/features/complete-setup'
import { Button, Notice, Typography, pageStyles } from '@/shared/ui'

export function FirstUseSetupGate({ children }: { children: ReactNode }) {
  const { user } = useSession()
  return (
    <AccountGate key={user?.id ?? ''} ownerId={user?.id ?? ''}>
      {children}
    </AccountGate>
  )
}
function AccountGate({ ownerId, children }: { ownerId: string; children: ReactNode }) {
  const { t } = useTranslation('creation')
  const setup = useSetupAvailability(ownerId)
  useEffect(() => {
    if (setup.status === 'ready' && !setup.needed && !setup.progress.completed) setup.acknowledge()
  }, [setup])
  if (setup.status === 'checking')
    return (
      <main className={pageStyles({ className: 'flex flex-1 items-center justify-center' })}>
        <Typography variant="body" role="status">
          {t('setup.checking')}
        </Typography>
      </main>
    )
  if (setup.status === 'failed')
    return (
      <main className={pageStyles({ className: 'flex flex-1 flex-col justify-center gap-6' })}>
        <Notice tone="danger" role="alert">
          {t('setup.loadFailed')}
        </Notice>
        <div className="flex flex-wrap gap-3">
          <Button variant="cta" onClick={setup.retry}>
            {t('setup.retry')}
          </Button>
          <Button variant="ghost" onClick={setup.acknowledge}>
            {t('setup.continue')}
          </Button>
        </div>
      </main>
    )
  if (setup.needed) return <Navigate to="/setup" search={{ restart: false }} replace />
  return children
}
