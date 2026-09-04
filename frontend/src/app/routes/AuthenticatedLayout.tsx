import type { ComponentType } from 'react'
import { Bot, FileText, ListChecks, Quote, Send, Target } from 'lucide-react'
import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { Logo, typographyStyles } from '@/shared/ui'
import { AccountMenu } from '@/widgets/account-menu'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { endSession } from '../model/end-session'

/** The app's destinations, in one list so the phone tab bar, the laptop header and the desk
 *  sidebar cannot drift. Every destination stays visible rather than moving behind a menu — NN/g
 *  measured a >20% drop in discoverability once navigation is hidden. An ordinary account sees
 *  five, which is the top of the 3–5 a bottom bar is designed for; the operator sees six, and at
 *  320px that is 53px per tab against the 44px minimum, so the targets still hold. A seventh would
 *  have to displace one of these rather than be squeezed in beside them. */
const DESTINATIONS: ReadonlyArray<{
  to: string
  labelKey: 'posts' | 'voices' | 'purposes' | 'guidelines' | 'models' | 'publishingAgents'
  icon: ComponentType<{ className?: string }>
  /** The tier this destination requires. Publishing runs through OUR paired agent and OUR
   *  infrastructure ([I1]), so it is the operator's surface — and the server refuses every one
   *  of its procedures to anyone else, which is what makes hiding it honest rather than a tease. */
  masterOnly?: true
}> = [
  { to: '/posts', labelKey: 'posts', icon: FileText },
  { to: '/voices', labelKey: 'voices', icon: Quote },
  { to: '/purposes', labelKey: 'purposes', icon: Target },
  { to: '/guidelines', labelKey: 'guidelines', icon: ListChecks },
  { to: '/ai-models', labelKey: 'models', icon: Bot },
  { to: '/publishing-agents', labelKey: 'publishingAgents', icon: Send, masterOnly: true },
]

// Nav labels take their size from the type roles and their colour from the link tokens — the
// merge resolves toward the link colour. The active state adds emphasis, which the §3 roles do
// not model for controls, hence the pragma'd raw weight.
const headerNavLinkStyles = typographyStyles({
  variant: 'label',
  className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2',
})
const phoneNavLinkStyles = typographyStyles({
  variant: 'label',
  className:
    'text-link-fg active:bg-row-bg-active flex min-h-14 flex-1 flex-col items-center justify-center gap-1',
})
// The desk row is a ROW, not a word: it owns the sidebar's full width, so the whole strip is the
// target and the current state can be a plane rather than a colour on four characters. Same icon
// as the phone tab so the two shapes read as one navigation, `px-4` against the sidebar's own
// `px-4` inset so a row is never inset less than the gutter it sits in (§4.2), which also lands
// the icon column exactly under the wordmark in the bar above it.
//
// It carries NO background of its own — the two it can take are split across `inactiveProps` and
// `activeProps` instead of `hover:` plus a base class. Both would emit a `hover:bg-*` rule on the
// same element, and which one wins is Tailwind's own stylesheet order rather than anything this
// file says, so the current row would flicker to the hover plane under the pointer. Splitting it
// means the current row simply has no hover rule to lose to.
//
// `row-bg-*` cannot serve here and that is the point of §4.5's third plane: those two are
// calibrated for a row on `surface-base` (hover lifts to `raised`, press sinks to `recessed`), and
// on a `recessed` rail the press plane IS the rail and the hover plane is the header. The rail
// walks its own three steps instead — recessed at rest, `base` under the pointer, `raised` for the
// current destination, which is the header's plane and so the brightest thing in the column.
const deskNavLinkStyles = typographyStyles({
  variant: 'label',
  className: 'text-link-fg flex min-h-11 items-center gap-3 rounded-md px-4',
})

/** The shell every signed-in screen renders inside.
 *
 *  The header lives here rather than in RootLayout so "only shown when there is a
 *  session" is structural: this component is the authenticated route's own component, so
 *  its guard has already resolved the session before it renders. Putting it at the root
 *  would make /login issue a GetMe it does not need.
 *
 *  THREE shapes, one per pointer (design-language §1.5, §4.3, §4.5).
 *
 *  On a 430x932 phone held one-handed the top ~370px is a re-grip, not a tap, and a top bar that
 *  is not sticky is off-screen entirely after one swipe — so navigation moves to a fixed bottom bar
 *  in the thumb's band and the top bar keeps only the brand and the session controls. Because the
 *  bottom bar is always present, the top bar does NOT need to be sticky on a phone: it scrolls away
 *  and gives its 56px back to the content.
 *
 *  From `sm:` up the pointer is a mouse, the reach argument disappears, and the familiar sticky top
 *  nav returns.
 *
 *  From `lg:` up there is a second axis to spend. A horizontal strip of five links in a 1440px bar
 *  leaves the destinations huddled in the top-left corner and the content below it capped at a
 *  phone width, which is the shape that read as "a mobile page centred on a desk". So the
 *  destinations move into a persistent left sidebar — the app-shell every comparable product uses
 *  — where a vertical list is scanned top-to-bottom (measurably faster to target than a horizontal
 *  one), the active row is a plane rather than a word, and the space they vacate goes to the
 *  content column via `pageStyles`. The top bar STAYS at this width rather than folding into the
 *  sidebar: the brand, the theme, the locale and the account exist exactly once in the tree, so
 *  there is one Logo and one AccountMenu at every width instead of two of each fighting over which
 *  is visible. */
export function AuthenticatedLayout() {
  const { t } = useTranslation(['nav'])
  const { user } = useSession()
  const navigate = useNavigate()
  // Five destinations for an ordinary account, six for the operator — every shape keeps every
  // destination visible.
  const destinations = DESTINATIONS.filter(
    (destination) => !destination.masterOnly || user?.plan === 'master',
  )

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <header className="bg-surface-raised flex min-h-14 items-center justify-between gap-2 px-4 sm:sticky sm:top-0 sm:z-20 sm:min-h-16 sm:px-6">
        <div className="flex min-w-0 items-center gap-1">
          {/* The wordmark IS the home link — no separate text label beside it, which would give
              the same destination two accessible names. Sized by height: `h-6` puts the
              lowercase band at ~14px, level with the nav links it sits next to, and `min-h-11`
              keeps the target legal (§4.1) without the mark growing to fill it. At `lg:` it is
              also the sidebar's masthead: the bar and the rail meet at the same left edge, so the
              mark sits over the column it names. */}
          <Link
            to="/posts"
            className="inline-flex min-h-11 items-center px-2"
            aria-label={t('home', { ns: 'nav' })}
          >
            <Logo className="h-6" />
          </Link>
          {/* The same destinations as the tab bar, for the laptop only: from `lg:` up they are the
              sidebar's job and a second copy in the bar would be the same five links twice. */}
          <nav
            className="hidden items-center gap-1 sm:flex lg:hidden"
            aria-label={t('primary', { ns: 'nav' })}
          >
            {destinations.map((destination) => (
              <Link
                key={destination.to}
                to={destination.to}
                // `px-2` is not decoration: `min-h-11` sets only the HEIGHT, and '말투' is two
                // Hangul at 14px — a 28x44 target without it (§4.1).
                className={headerNavLinkStyles}
                activeProps={{
                  className: 'text-link-fg-current font-medium', // style-escape: active-state emphasis on a nav control, not a §3 text role
                  'aria-current': 'page',
                }}
              >
                {t(destination.labelKey, { ns: 'nav' })}
              </Link>
            ))}
          </nav>
        </div>
        {/* Three quiet session controls, viewport-side last: theme, locale, account. Their
            right-aligned panels then land inside the 320px shell gutters (§8.5). */}
        <div className="flex shrink-0 items-center gap-2">
          <InterfacePreferences />
          <AccountMenu
            // Where logout lands is the shell's decision, not the widget's. The widget resolves
            // the Logout mutation first, so by the time this runs the cookie is gone and dropping
            // the cache cannot race the guard into reading a stale session.
            onLoggedOut={() => {
              endSession()
              void navigate({ to: '/login', replace: true })
            }}
          />
        </div>
      </header>
      {/* The desk's two columns. One flex row from `lg:` up, a plain single column below it, so
          the sidebar costs nothing at the widths that do not render it. */}
      <div className="flex flex-1 flex-col lg:flex-row">
        {/* Hangs from the header's bottom edge and holds it: `h-sidebar` is the viewport minus that
            bar, which is what makes `sticky` engage instead of the rail scrolling away with the
            page. It does not compete with the document's scroll (§4.4) — six rows never fill it —
            but `overflow-y-auto` is there so a future seventh cannot push the last row off the
            bottom of the screen.

            `surface-recessed`, one step BELOW the page. The rail and the bar are the app's two
            pieces of chrome and they meet at a corner, so painting both `raised` made them one
            L-shaped slab with no edge between them — and left the rows' hover plane, which is
            `raised`, invisible against their own background. Three planes instead of one: the rail
            sinks behind the page, the page is the page, and the bar floats over both of them
            (§4.5). */}
        <aside className="bg-surface-recessed lg:top-header lg:h-sidebar hidden shrink-0 lg:sticky lg:flex lg:w-60 lg:flex-col lg:overflow-y-auto lg:px-4 lg:py-4">
          <nav className="flex flex-col gap-0.5" aria-label={t('primary', { ns: 'nav' })}>
            {destinations.map((destination) => (
              <Link
                key={destination.to}
                to={destination.to}
                className={deskNavLinkStyles}
                inactiveProps={{ className: 'hover:bg-surface-base hover:text-link-fg-hover' }}
                activeProps={{
                  className: 'bg-surface-raised text-link-fg-current font-medium', // style-escape: current-destination emphasis on a nav control, not a §3 text role
                  'aria-current': 'page',
                }}
              >
                <destination.icon className="size-5 shrink-0" />
                {t(destination.labelKey, { ns: 'nav' })}
              </Link>
            ))}
          </nav>
        </aside>
        {/* `pb-nav` reserves the tab bar's height plus the home indicator, so the last thing on
            every page stays reachable instead of sitting under fixed chrome. The flex column is
            what lets a page opt into filling the viewport (`flex-1` on its own `main`) so a docked
            ActionBar lands at the bottom even when its content is short — `sticky` alone cannot do
            that, because it only engages once the element would otherwise scroll out of view. A
            page that does not opt in is unaffected: it simply does not grow. `min-w-0` is what
            keeps a wide table or a long model id inside the column instead of pushing the sidebar
            off the screen — a flex item's default `min-width: auto` refuses to shrink past its
            content. */}
        <div className="pb-nav flex min-w-0 flex-1 flex-col sm:pb-0">
          <Outlet />
        </div>
      </div>
      <nav
        className="bg-surface-raised pb-safe-b fixed inset-x-0 bottom-0 z-30 flex shadow-lg sm:hidden"
        aria-label={t('primary', { ns: 'nav' })}
      >
        {destinations.map((destination) => (
          <Link
            key={destination.to}
            to={destination.to}
            // Icon AND label on every destination: an icon alone is a guess, and these are not
            // conventional enough to carry meaning on their own.
            className={phoneNavLinkStyles}
            activeProps={{
              className: 'text-link-fg-current font-medium', // style-escape: active-state emphasis on a nav control, not a §3 text role
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
