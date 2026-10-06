import { useInitializeDefaultSelections } from '@/entities/model-catalog'
import { useSession } from '@/entities/session'
import { useEffect, useReducer } from 'react'
import { Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Menu as MenuIcon, X, ArrowUpRight } from 'lucide-react'
import { clsx } from 'clsx'
import { takePendingGift } from '@/features/redeem-voucher'
import { Button, Logo, Sheet, PromoStage, Typography, typographyStyles } from '@/shared/ui'
import { AccountMenu } from '@/widgets/account-menu'
import { CreditBadge } from '@/widgets/credit-badge'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { endSession } from '../model/end-session'
import { navigationMenuTransition } from '../model/navigation-menu'
import { currentDestination, DESTINATIONS } from './navigation'

export function AuthenticatedLayout() {
  const { t } = useTranslation('nav')
  const { user } = useSession()
  const defaults = useInitializeDefaultSelections(user?.id ?? '')
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const [menu, send] = useReducer(navigationMenuTransition, 'closed')
  const current = currentDestination(pathname)
  const immersive = pathname === '/plans'
  const quiet = pathname === '/' || pathname === '/setup'

  useEffect(() => {
    const token = takePendingGift()
    if (token) void navigate({ to: '/gift/$token', params: { token }, replace: true })
  }, [navigate])
  useEffect(() => {
    send('navigate')
  }, [pathname])

  return (
    <div
      data-plans-shell={immersive || undefined}
      className="bg-surface-base text-content-primary relative isolate flex min-h-full flex-col"
    >
      {immersive && <PromoStage viewport />}
      <header
        className={clsx(
          'min-h-header sticky top-0 z-30 has-[[aria-expanded=true]]:z-40',
          immersive ? 'bg-surface-raised/70 backdrop-blur-xl' : 'bg-surface-base',
        )}
      >
        <div className="h-header mx-auto flex w-full max-w-7xl items-center gap-2 px-4 sm:px-6 lg:px-8">
          <Link
            to="/"
            aria-label={t('home')}
            className="inline-flex min-h-11 min-w-0 flex-1 items-center"
          >
            <Logo className="h-6 max-w-full" />
          </Link>
          <div className="ml-auto flex min-w-0 items-center gap-2">
            {!quiet && <CreditBadge />}
            <Button
              variant="ghost"
              size="icon"
              aria-label={t('sidebar.open')}
              aria-expanded={menu === 'open'}
              aria-haspopup="dialog"
              onClick={() => send('open')}
            >
              <MenuIcon aria-hidden="true" className="size-5" />
            </Button>
            <div className="hidden lg:flex">
              <InterfacePreferences />
            </div>
            <AccountMenu
              preferences={<InterfacePreferences layout="rows" />}
              onLoggedOut={() => {
                endSession()
                void navigate({ to: '/login', replace: true })
              }}
            />
          </div>
        </div>
      </header>
      <div className="flex min-w-0 flex-1 flex-col">
        {defaults.isError && (
          <div className="mx-auto w-full max-w-7xl px-4 py-3 sm:px-6 lg:px-8">
            <Typography variant="body" role="status">
              {t('aiPreparingFailed')}{' '}
              <Button variant="ghost" onClick={defaults.retry}>
                {t('retryAI')}
              </Button>
            </Typography>
          </div>
        )}
        <Outlet />
      </div>
      <Sheet
        open={menu === 'open'}
        onClose={() => send('close')}
        labelledBy="navigation-menu-title"
        header={
          <div className="flex items-center justify-between gap-4">
            <Typography variant="title" id="navigation-menu-title">
              {t('menu')}
            </Typography>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t('sidebar.close')}
              onClick={() => send('close')}
            >
              <X aria-hidden="true" className="size-5" />
            </Button>
          </div>
        }
      >
        <nav aria-label={t('primary')} className="flex flex-col gap-2">
          {DESTINATIONS.map(({ to, labelKey, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              aria-current={current === to ? 'page' : undefined}
              onClick={() => send('navigate')}
              className={typographyStyles({
                variant: 'body',
                className: clsx(
                  'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-16 items-center gap-4 rounded-lg px-4 py-3',
                  current === to && 'bg-surface-raised text-link-fg-current',
                ),
              })}
            >
              <Icon aria-hidden="true" className="size-5 shrink-0" />
              <span>{t(labelKey)}</span>
              <ArrowUpRight aria-hidden="true" className="ml-auto size-4 shrink-0" />
            </Link>
          ))}
        </nav>
      </Sheet>
    </div>
  )
}
