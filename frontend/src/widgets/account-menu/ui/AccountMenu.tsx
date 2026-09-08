import type { ComponentType, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { CreditCard, LogOut, Settings, ShieldCheck, Sparkles, UserRound } from 'lucide-react'
import { useMyPlan, planLabel, type MyPlan } from '@/entities/plan'
import { useLogout, useSession } from '@/entities/session'
import {
  AppFailureMessage,
  Badge,
  Button,
  Meter,
  Notice,
  Popover,
  Typography,
  typographyStyles,
} from '@/shared/ui'

/** The header's one session control: an avatar that opens the account material — who is signed
 *  in and on which tier, the balance behind it, the way to account, plans, billing and (for the
 *  operator) administration, and logout. Collapsing these into a popover keeps the 320px header
 *  to three quiet controls without hiding any session fact more than one tap away.
 *
 *  The panel is a component of its own so it MOUNTS when the popover opens: the balance query
 *  gains an observer at that moment, which is what makes "refreshed when opened" fall out of the
 *  query rather than out of a timer. The shell already reads the same cache entry for the header
 *  figure, so opening adds no request when that read is fresh. */
export function AccountMenu({ onLoggedOut }: { onLoggedOut: () => void }) {
  const { t } = useTranslation('auth')

  return (
    <Popover
      label={t('account.label')}
      triggerLabel={
        // The session model has no profile picture, so the header uses one stable profile glyph
        // instead of turning an arbitrary account-id character into an avatar.
        <UserRound aria-hidden="true" className="size-5" />
      }
      triggerClassName="size-10 rounded-full px-0 pointer-coarse:size-11"
      placement="below"
    >
      {(close) => <AccountPanel close={close} onLoggedOut={onLoggedOut} />}
    </Popover>
  )
}

/** One destination row in the panel: an icon, its label, and the row's own hover plane.
 *
 *  The rows are what makes this read as a menu rather than a page: the whole strip is the
 *  target, so a row no longer needs a column of empty space around a 14px word to be pressable;
 *  it rests at the 36px menu-row floor under a mouse and 44px under a thumb (THEME-23). `-mx-3` widens the strip 12px into the panel's padding on each side so the label
 *  stays aligned with the identity block above while the plane under the pointer reaches almost
 *  to the panel's edge, the way a menu row does. Type comes from the `label` role and colour from
 *  the link tokens (design-language §3, §9). */
const rowStyles = typographyStyles({
  variant: 'label',
  className:
    'text-link-fg hover:text-link-fg-hover hover:bg-row-bg-hover active:bg-row-bg-active -mx-3 flex min-h-9 items-center gap-3 rounded-md px-3 pointer-coarse:min-h-11',
})

function MenuRow({
  to,
  icon: Icon,
  close,
  children,
}: {
  to: string
  icon: ComponentType<{ className?: string }>
  close: () => void
  children: ReactNode
}) {
  return (
    <Link to={to} onClick={close} className={rowStyles}>
      <Icon aria-hidden="true" className="size-4 shrink-0" />
      {children}
    </Link>
  )
}

/** Everything behind the avatar, in three groups a hairline apart: identity and balance, the
 *  destinations, and the one action.
 *
 *  Logout runs here, but where a SUCCESSFUL logout lands stays the shell's decision via
 *  `onLoggedOut` — the shell owns the session cache drop and the navigation, and awaiting the
 *  mutation first is what keeps the guard from reading a stale session. A FAILED logout leaves
 *  the cookie valid, so the popover stays open and says so where the user is already looking
 *  (design-language §4.3) instead of pretending the session ended. */
function AccountPanel({ close, onLoggedOut }: { close: () => void; onLoggedOut: () => void }) {
  const { t } = useTranslation(['auth', 'billing', 'common', 'plans'])
  const { user } = useSession()
  const { myPlan, isPending, isError } = useMyPlan()
  const logout = useLogout()

  const onLogout = async () => {
    try {
      await logout.mutateAsync({})
    } catch {
      return
    }
    onLoggedOut()
  }

  return (
    <div className="divide-divider grid divide-y">
      <section className="grid gap-3 pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="grid min-w-0 gap-0.5">
            <Typography variant="meta" as="p">
              {t('account.signedInAs', { ns: 'auth' })}
            </Typography>
            <Typography variant="label" as="p" mono className="text-content-primary break-words">
              {user?.id}
            </Typography>
          </div>
          {/* The tier, in the panel's top-right corner: the operator reads as the operator before
              anything else, and every other account sees its plan where a status chip belongs
              rather than halfway down a list. The name comes from GetMyPlan like every other
              figure here. */}
          {myPlan && <Badge tone="accent">{planLabel(myPlan.plan)}</Badge>}
        </div>
        <CreditSummary myPlan={myPlan} isPending={isPending} isError={isError} />
      </section>

      <nav aria-label={t('links.more', { ns: 'auth' })} className="grid gap-0.5 py-2">
        <MenuRow to="/account" icon={Settings} close={close}>
          {t('accountSettings.heading', { ns: 'auth' })}
        </MenuRow>
        {/* The ladder is worth reaching from every tier, including the operator's: hiding the one
            link to it behind "has a meter" was why `/plans` went unreachable for a master account
            (QUOTA-27). */}
        <MenuRow to="/plans" icon={Sparkles} close={close}>
          {t('balance.viewPlans', { ns: 'plans' })}
        </MenuRow>
        <MenuRow to="/billing" icon={CreditCard} close={close}>
          {t('nav', { ns: 'billing' })}
        </MenuRow>
        {myPlan?.plan === 'master' && (
          <MenuRow to="/admin" icon={ShieldCheck} close={close}>
            {t('admin.nav', { ns: 'plans' })}
          </MenuRow>
        )}
      </nav>

      <div className="grid gap-2 pt-2">
        {/* The same row shape as the destinations above — widened into the padding, icon first —
            so the one action reads as the menu's last row rather than as a form's submit. */}
        <Button
          variant="ghost"
          className="-mx-3 min-h-9 justify-start gap-3 px-3 pointer-coarse:min-h-11"
          onClick={() => void onLogout()}
          pending={logout.isPending}
        >
          <LogOut aria-hidden="true" className="size-4 shrink-0" />
          {t('action.logout', { ns: 'common' })}
        </Button>
        {logout.failure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={logout.failure} />
            <span>{t('logout.failed', { ns: 'auth' })}</span>
          </Notice>
        )}
      </div>
    </div>
  )
}

/** Which label names a lot. A lookup rather than a ternary because there are three kinds
 *  now (QUOTA-12) and a ternary would silently call a purchase a bonus. */
const LOT_LABEL = { monthly: 'Monthly', bonus: 'Bonus', purchased: 'Purchased' } as const

/** The balance and the lots behind it.
 *
 *  Every number comes from GetMyPlan: the grant table and the charge formula are server-owned
 *  product rules, so a figure hardcoded here would be a second source of truth that silently
 *  goes stale. The meter's own label names the block — a heading above it said the same two
 *  words a second time, one row apart. */
function CreditSummary({
  myPlan,
  isPending,
  isError,
}: {
  myPlan: MyPlan | undefined
  isPending: boolean
  isError: boolean
}) {
  const { t } = useTranslation('plans')

  if (isPending)
    return (
      <Typography variant="body" className="text-content-tertiary">
        {t('balance.loading')}
      </Typography>
    )
  if (isError || !myPlan)
    return (
      <Typography variant="body" role="status" className="text-content-tertiary">
        {t('balance.loadFailed')}
      </Typography>
    )

  const { balance } = myPlan
  // An unlimited account is stated, not drawn: `Meter` with no ceiling renders the label and the
  // figure with no bar, because an empty track next to "unlimited" reads as "none left".
  if (balance.unlimited)
    return (
      <Meter label={t('balance.heading')} value={0} max={0} valueText={t('balance.unlimited')} />
    )

  return (
    <div className="grid gap-2">
      {/* The meter fills against everything the account was GRANTED, not against the monthly
          figure alone: a signup bonus makes the balance larger than one month's grant, and a bar
          that read 100/50 would be nonsense. */}
      <Meter
        label={t('balance.heading')}
        value={balance.credits}
        max={balance.lots.reduce((total, lot) => total + lot.granted, 0) || balance.monthlyGrant}
        valueText={t('balance.credits', { count: balance.credits })}
        note={balance.renewsAt ? t('balance.renews', { at: balance.renewsAt }) : undefined}
      />
      {/* The lots behind the total, in the order they will be spent. A single number cannot say
          that half the balance lapses at the month boundary and half does not, which is the one
          thing a bonus holder needs to know. */}
      <ul className="grid gap-0.5">
        {balance.lots.map((lot, index) => (
          <li key={`${lot.kind}-${index}`} className="flex items-baseline justify-between gap-2">
            <Typography variant="meta">
              {t(`balance.lot${LOT_LABEL[lot.kind]}`)}
              {' · '}
              {lot.expiresAt
                ? t('balance.lotExpires', { at: lot.expiresAt })
                : t('balance.lotNoExpiry')}
            </Typography>
            <Typography variant="meta">
              {t('balance.ofGrant', { remaining: lot.remaining, granted: lot.granted })}
            </Typography>
          </li>
        ))}
      </ul>
    </div>
  )
}
