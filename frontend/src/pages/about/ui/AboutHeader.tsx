import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { buttonStyles, Logo } from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

/** The public header: wordmark, the way in, and the shared preferences — ONE row at every width
 *  (MARKETING-11).
 *
 *  It used to stack on a phone, the wordmark over a centred pair of Get started and Login, which
 *  left the wordmark flush against the top edge and two controls reading as two buttons. Now the
 *  bar is the same shape the app's own header has: the wordmark at the gutter, the controls
 *  viewport-side, everything centred in the bar's height. The quiet Login link lives in the hero
 *  under the access sentence (MARKETING-6), where a returning visitor is already reading about
 *  the account path, so the header carries exactly one action.
 *
 *  Sticky because Get started is this page's ONE filled CTA and the page is long — one repeated
 *  at the bottom would be the second one MARKETING-6 forbids, so the single one stays reachable
 *  instead. `pt-safe-t` is what a notched phone in landscape needs on its leading edge.
 *
 *  The wordmark is a step smaller on a phone than in the app: at 320px the row holds the mark,
 *  the CTA and two icon buttons at their touch size, and `h-6` is the 5px that would push it into
 *  horizontal scroll. */
export function AboutHeader() {
  const { t } = useTranslation('marketing')
  return (
    <header className="bg-surface-raised pt-safe-t sticky top-0 z-20 flex min-h-14 items-center justify-between gap-3 px-4 sm:min-h-16 sm:px-6">
      {/* The wordmark is the page's own identity here, not a link: `/about` IS this page, and a
          link to the current route is a dead control. */}
      <Logo className="h-5 shrink-0 sm:h-6" />
      <div className="flex shrink-0 items-center gap-1 sm:gap-2">
        <Link to="/signup" className={buttonStyles({ variant: 'cta', className: 'shrink-0' })}>
          {t('header.getStarted')}
        </Link>
        {/* The two right-aligned menu panels stay viewport-side last. At 320px, putting the
            action after them pushed the theme trigger far enough left that its 176px panel
            crossed the viewport edge even though the header row itself still fit. */}
        <InterfacePreferences />
      </div>
    </header>
  )
}
