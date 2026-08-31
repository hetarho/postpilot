import { useTranslation } from 'react-i18next'

/** Shown only when a route takes longer than TanStack's 1 s `defaultPendingMs`, so a fast
 *  connection never sees it. Route splitting made a silent wait possible where none existed:
 *  the router awaits a route's chunk before committing the navigation, so without this the
 *  tap would leave the previous screen sitting there with nothing to say it was heard. */
export function RoutePending() {
  const { t } = useTranslation('common')
  return (
    <p role="status" className="text-content-tertiary mt-8 text-sm">
      {t('state.loading')}
    </p>
  )
}
