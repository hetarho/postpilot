import type { ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  ChevronRight,
  CreditCard,
  RotateCcw,
  ShieldCheck,
  Ticket,
  UserRound,
  Zap,
} from 'lucide-react'
import { useSession } from '@/entities/session'
import { planLabel, useMyPlan } from '@/entities/plan'
import {
  ListGroup,
  ListRowIcon,
  Typography,
  buttonStyles,
  cardStyles,
  listRowStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

type Icon = ComponentType<{ className?: string }>

/** Guide: 나 — the owner's own tab. It opens on what they most often come here for, the credits
 *  left, as one big figure with ONE 충전 action; under it the rest are iOS-style grouped rows, each
 *  drilling into a one-topic screen. What the owner taught the AI lives in 내 스타일 and model
 *  choice in AI 고르기, so nothing here is about writing. */
export function SettingsPage() {
  const { t } = useTranslation(['creation', 'plans'])
  const { user } = useSession()
  const { myPlan, isPending } = useMyPlan()
  const tier = planLabel(myPlan?.plan)
  const unlimited = myPlan?.balance.unlimited ?? false
  const figure = unlimited
    ? t('balance.unlimited', { ns: 'plans' })
    : myPlan && !isPending
      ? t('balance.credits', { ns: 'plans', count: myPlan.balance.credits })
      : '—'
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
      <Typography variant="largeTitle">{t('settings.title')}</Typography>

      {/* The hero: what is left to spend, the plan it comes from, and the one way to get more. */}
      <section
        aria-labelledby="me-credits"
        className={cardStyles({
          padding: 'lg',
          className: 'bg-badge-accent-bg animate-step-in mt-6 flex flex-col gap-5',
        })}
      >
        <div className="flex flex-col gap-1">
          <h2 id="me-credits" className={typographyStyles({ variant: 'label' })}>
            {t('settings.credits')}
          </h2>
          <p className={typographyStyles({ variant: 'figure' })}>{figure}</p>
          <p className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}>
            {tier}
          </p>
        </div>
        <Link
          to="/billing"
          className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full' })}
        >
          <Zap aria-hidden="true" className="size-5" />
          {t('settings.charge')}
        </Link>
      </section>

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
