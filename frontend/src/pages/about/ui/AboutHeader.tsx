import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { isInAppPath } from '@/shared/lib'
import { buttonStyles, Logo, typographyStyles } from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

/** The public header: wordmark, the way in, Login, and the shared preferences.
 *
 *  Sticky because Get started is this page's ONE filled CTA and the page is long — one repeated
 *  at the bottom would be the second one plan 15 forbids, so the single one stays reachable
 *  instead. `pb-safe-b` is not needed here (it is top-anchored), but `pt-safe-t` is: on a notched
 *  phone in landscape the inset is on the leading edge.
 *
 *  Both actions stay router links because they navigate; the quiet Login link alone preserves
 *  the destination that led through this public page. */
export function AboutHeader() {
  const { t } = useTranslation('marketing')
  // Handed straight back to /login so a detour through this page does not cost the visitor the
  // destination their session expired on. Filtered here as well as there: an off-site value must
  // never survive a round trip through a public page.
  const { redirect } = useSearch({ from: '/about' })
  const carried = isInAppPath(redirect) ? redirect : undefined
  return (
    <header className="bg-surface-raised pt-safe-t sticky top-0 z-20 flex min-h-14 flex-col items-center justify-between gap-1 px-4 pb-1 sm:min-h-16 sm:flex-row sm:gap-2 sm:px-6 sm:pb-0">
      {/* The wordmark is the page's own identity here, not a link: `/about` IS this page, and a
          link to the current route is a dead control. */}
      <Logo className="h-6 shrink-0" />
      <div className="flex w-full min-w-0 items-center justify-center gap-1 sm:w-auto sm:justify-end sm:gap-2">
        <Link to="/signup" className={buttonStyles({ variant: 'cta', className: 'shrink-0' })}>
          {t('header.getStarted')}
        </Link>
        <Link
          to="/login"
          search={carried ? { redirect: carried } : {}}
          className={typographyStyles({
            variant: 'label',
            className:
              'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 shrink-0 items-center px-1 underline',
          })}
        >
          {t('header.login')}
        </Link>
        {/* Keep the two right-aligned menu panels viewport-side last. At 320px, putting actions
            after them pushed the theme trigger far enough left that its 176px panel crossed the
            viewport edge even though the header row itself still fit. */}
        <InterfacePreferences />
      </div>
    </header>
  )
}
