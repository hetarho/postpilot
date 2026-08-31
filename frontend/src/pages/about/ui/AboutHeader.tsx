import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { isInAppPath } from '@/shared/lib'
import { buttonStyles, Logo } from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

/** The public header: wordmark, the shared locale/theme preferences, and Login.
 *
 *  Sticky because Login is this page's ONE product action and the page is long — a CTA repeated
 *  at the bottom would be the second one plan 15 forbids, so the single one stays reachable
 *  instead. `pb-safe-b` is not needed here (it is top-anchored), but `pt-safe-t` is: on a notched
 *  phone in landscape the inset is on the leading edge.
 *
 *  Login is a router `Link` with the shared button contract, not a `Button`: it navigates, so it
 *  must stay an anchor for middle-click, copy-link and assistive technology. */
export function AboutHeader() {
  const { t } = useTranslation('marketing')
  // Handed straight back to /login so a detour through this page does not cost the visitor the
  // destination their session expired on. Filtered here as well as there: an off-site value must
  // never survive a round trip through a public page.
  const { redirect } = useSearch({ from: '/about' })
  const carried = isInAppPath(redirect) ? redirect : undefined
  return (
    <header className="bg-surface-raised pt-safe-t sticky top-0 z-20 flex min-h-14 items-center justify-between gap-2 px-4 sm:min-h-16 sm:px-6">
      {/* The wordmark is the page's own identity here, not a link: `/about` IS this page, and a
          link to the current route is a dead control. */}
      <Logo className="h-6 shrink-0" />
      <div className="flex min-w-0 shrink items-center gap-2">
        <InterfacePreferences />
        <Link
          to="/login"
          search={carried ? { redirect: carried } : {}}
          className={buttonStyles({ variant: 'cta', className: 'shrink-0' })}
        >
          {t('header.login')}
        </Link>
      </div>
    </header>
  )
}
