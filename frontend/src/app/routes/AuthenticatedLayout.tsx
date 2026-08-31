import type { ComponentType } from 'react'
import { Bot, FileText, Quote, Send, Target } from 'lucide-react'
import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useLogout, useSession } from '@/entities/session'
import { AppFailureMessage, Button, Logo, Notice } from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { endSession } from '../model/end-session'

/** The app's destinations, in one list so the phone tab bar and the desktop header cannot drift.
 *  Five is the top of the 3–5 a bottom bar is designed for, so every destination stays visible —
 *  NN/g measured a >20% drop in discoverability once navigation is hidden behind a menu. A sixth
 *  would have to displace one of these rather than be squeezed in beside them. */
const DESTINATIONS: ReadonlyArray<{
  to: string
  labelKey: 'posts' | 'voices' | 'purposes' | 'models' | 'publishingAgents'
  icon: ComponentType<{ className?: string }>
}> = [
  { to: '/posts', labelKey: 'posts', icon: FileText },
  { to: '/voices', labelKey: 'voices', icon: Quote },
  { to: '/purposes', labelKey: 'purposes', icon: Target },
  { to: '/ai-models', labelKey: 'models', icon: Bot },
  { to: '/publishing-agents', labelKey: 'publishingAgents', icon: Send },
]

/** The shell every signed-in screen renders inside.
 *
 *  The header lives here rather than in RootLayout so "only shown when there is a
 *  session" is structural: this component is the authenticated route's own component, so
 *  its guard has already resolved the session before it renders. Putting it at the root
 *  would make /login issue a GetMe it does not need.
 *
 *  The shape is different on a phone and that is deliberate (design-language §1.5, §4.3). On a
 *  430x932 phone held one-handed the top ~370px is a re-grip, not a tap, and a top bar that is not
 *  sticky is off-screen entirely after one swipe — so navigation moves to a fixed bottom bar in the
 *  thumb's band and the top bar keeps only the brand and the session control. Because the bottom
 *  bar is always present, the top bar does NOT need to be sticky on a phone: it scrolls away and
 *  gives its 56px back to the content. From `sm:` up the pointer is a mouse, the reach argument
 *  disappears, and the familiar sticky top nav returns. */
export function AuthenticatedLayout() {
  const { t } = useTranslation(['nav', 'auth', 'common'])
  const { user } = useSession()
  const logout = useLogout()
  const navigate = useNavigate()

  // Where logout lands is the shell's decision, not the entity's — so the navigation
  // lives here rather than in useLogout. Awaiting is what orders it correctly:
  // mutateAsync resolves only after the session cache has been dropped, and navigating
  // first would let the guard read the stale entry and send us right back.
  //
  // Navigating only on success is deliberate. A failed Logout leaves the cookie valid,
  // so leaving would be theatre: the guard on /login would find the live session and
  // bounce the user back in. Better to stay and say it did not work.
  const onLogout = async () => {
    try {
      await logout.mutateAsync({})
    } catch {
      return
    }
    endSession()
    void navigate({ to: '/login', replace: true })
  }

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <header className="bg-surface-raised flex min-h-14 items-center justify-between gap-2 px-4 sm:sticky sm:top-0 sm:z-20 sm:min-h-16 sm:px-6">
        <div className="flex min-w-0 items-center gap-1">
          {/* The wordmark IS the home link — no separate text label beside it, which would give
              the same destination two accessible names. Sized by height: `h-6` puts the
              lowercase band at ~14px, level with the nav links it sits next to, and `min-h-11`
              keeps the target legal (§4.1) without the mark growing to fill it. */}
          <Link
            to="/posts"
            className="inline-flex min-h-11 items-center px-2"
            aria-label={t('home', { ns: 'nav' })}
          >
            <Logo className="h-6" />
          </Link>
          {/* The same destinations as the tab bar, for the pointer breakpoint only. */}
          <nav
            className="hidden items-center gap-1 sm:flex"
            aria-label={t('primary', { ns: 'nav' })}
          >
            {DESTINATIONS.map((destination) => (
              <Link
                key={destination.to}
                to={destination.to}
                // `px-2` is not decoration: `min-h-11` sets only the HEIGHT, and '말투' is two
                // Hangul at 14px — a 28x44 target without it (§4.1).
                className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 text-sm"
                activeProps={{
                  className: 'text-link-fg-current font-medium',
                  'aria-current': 'page',
                }}
              >
                {t(destination.labelKey, { ns: 'nav' })}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <span className="text-content-tertiary hidden font-mono text-xs sm:inline">
            {user?.id}
          </span>
          {/* `secondary`, not `ghost`: a ghost button's only fill lives behind `hover:`, which
              Tailwind compiles to `@media (hover: hover)` and a phone never matches — so the one
              control in the header used to render as text (§6). */}
          <Button variant="secondary" onClick={() => void onLogout()} pending={logout.isPending}>
            {t('action.logout', { ns: 'common' })}
          </Button>
          {/* Keep preferences at the viewport-side edge. Its right-aligned 18rem panel then lands
              exactly inside the 320px shell gutters instead of extending left by Logout's width. */}
          <InterfacePreferences />
        </div>
      </header>
      {logout.failure && (
        <Notice tone="danger" role="alert" className="rounded-none px-4 sm:px-6">
          <AppFailureMessage failure={logout.failure} />
          <span>{t('logout.failed', { ns: 'auth' })}</span>
        </Notice>
      )}
      {/* `pb-nav` reserves the tab bar's height plus the home indicator, so the last thing on every
          page stays reachable instead of sitting under fixed chrome. The flex column is what lets a
          page opt into filling the viewport (`flex-1` on its own `main`) so a docked ActionBar
          lands at the bottom even when its content is short — `sticky` alone cannot do that,
          because it only engages once the element would otherwise scroll out of view. A page that
          does not opt in is unaffected: it simply does not grow. */}
      <div className="pb-nav flex flex-1 flex-col sm:pb-0">
        <Outlet />
      </div>
      <nav
        className="bg-surface-raised pb-safe-b fixed inset-x-0 bottom-0 z-30 flex shadow-lg sm:hidden"
        aria-label={t('primary', { ns: 'nav' })}
      >
        {DESTINATIONS.map((destination) => (
          <Link
            key={destination.to}
            to={destination.to}
            // Icon AND label on every destination: an icon alone is a guess, and these three are
            // not conventional enough to carry meaning on their own.
            className="text-link-fg active:bg-row-bg-active flex min-h-14 flex-1 flex-col items-center justify-center gap-1 text-xs"
            activeProps={{
              className: 'text-link-fg-current font-medium',
              'aria-current': 'page',
            }}
          >
            <destination.icon className="size-5" />
            {t(destination.labelKey, { ns: 'nav' })}
          </Link>
        ))}
      </nav>
    </div>
  )
}
