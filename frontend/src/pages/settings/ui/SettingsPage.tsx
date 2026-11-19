import type { ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight, CreditCard, RotateCcw, ShieldCheck, Ticket, UserRound } from 'lucide-react'
import { useSession } from '@/entities/session'
import { planLabel, useMyPlan } from '@/entities/plan'
import {
  ListGroup,
  ListRowIcon,
  TitleRow,
  Typography,
  cardStyles,
  listRowStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { CreditsCard } from '@/widgets/credits-card'
import { InterfacePreferences } from '@/widgets/interface-preferences'

type Icon = ComponentType<{ className?: string }>

/** 설정, opened from the account menu (THEME-38): the shared credits card first (QUOTA-75), then
 *  grouped rows, each drilling into a one-topic screen. */
export function SettingsPage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const { myPlan } = useMyPlan()
  const tier = planLabel(myPlan?.plan)
  const row = (
    to: string,
    label: string,
    icon: Icon,
    value?: string,
    search?: Record<string, unknown>,
  ) => (
    <Link to={to} search={search} className={listRowStyles()}>
      <ListRowIcon icon={icon} />
      <span className="min-w-0 flex-1">{label}</span>
      {value && (
        <span className={typographyStyles({ variant: 'label', className: 'shrink-0' })}>
          {value}
        </span>
      )}
      <ChevronRight aria-hidden="true" className="text-content-tertiary size-5 shrink-0" />
    </Link>
  )
  return (
    <main className={pageStyles({ width: 'prose', className: 'pt-4 sm:pt-10' })}>
      <TitleRow>
        <Typography variant="largeTitle">{t('settings.title')}</Typography>
      </TitleRow>

      <CreditsCard className="mt-6" />

      <div className="mt-10 flex flex-col gap-8">
        <nav aria-labelledby="settings-plan">
          <ListGroup heading={t('settings.planGroup')} headingId="settings-plan">
            <li>{row('/plans', t('settings.plans'), Ticket, tier)}</li>
            <li>{row('/billing', t('settings.billing'), CreditCard)}</li>
          </ListGroup>
        </nav>
        <nav aria-labelledby="settings-account">
          <ListGroup heading={t('settings.accountGroup')} headingId="settings-account">
            <li>{row('/account', t('settings.account'), UserRound)}</li>
            <li>{row('/setup', t('setup.restart'), RotateCcw, undefined, { restart: true })}</li>
            {user?.plan === 'master' && <li>{row('/admin', t('settings.admin'), ShieldCheck)}</li>}
          </ListGroup>
        </nav>
        <section aria-labelledby="settings-screen" className="min-w-0">
          <h2
            id="settings-screen"
            className={typographyStyles({ variant: 'groupLabel', className: 'mb-2 px-4' })}
          >
            {t('settings.screen')}
          </h2>
          <div className={cardStyles({ padding: 'sm', className: 'shadow-sm' })}>
            <InterfacePreferences layout="rows" />
          </div>
        </section>
      </div>
    </main>
  )
}
