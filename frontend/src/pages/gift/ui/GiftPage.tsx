import { useEffect } from 'react'
import { Link, useLoaderData, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useGift, type VoucherState } from '@/entities/voucher'
import { RedeemVoucher, clearPendingGift } from '@/features/redeem-voucher'
import { AppFailureMessage, Notice, Typography, buttonStyles } from '@/shared/ui'

const UNAVAILABLE_HEADING = {
  redeemed: 'gift.redeemed',
  expired: 'gift.expired',
  revoked: 'gift.revoked',
  unknown: 'gift.notFound',
} as const satisfies Record<Exclude<VoucherState, 'redeemable'>, string>

/** The public page behind a gift link (GIFT-8).
 *
 *  Anyone sees what the link holds without an account; redeeming is an explicit action and
 *  never a side effect of opening the page (GIFT-9). */
export function GiftPage() {
  const { t } = useTranslation('plans')
  const { token } = useParams({ from: '/gift/$token' })
  const signedIn = useLoaderData({ from: '/gift/$token' })
  const { gift, isPending, failure } = useGift(token)
  const notFound = failure?.reason === 'VOUCHER_NOT_FOUND'
  const terminal = notFound || (gift !== undefined && gift.state !== 'redeemable')

  // The page is where a pending gift was headed; once here signed in, or once the link can
  // no longer be redeemed, there is nothing left to hand back.
  useEffect(() => {
    if (signedIn || terminal) clearPendingGift()
  }, [signedIn, terminal])

  return (
    <main className="bg-surface-base text-content-primary flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <section className="w-full max-w-sm text-center" aria-labelledby="gift-title">
        {isPending && (
          <Typography variant="body" as="h1" id="gift-title" className="text-content-secondary">
            {t('gift.loading')}
          </Typography>
        )}
        {!isPending && gift?.state === 'redeemable' && (
          <>
            <Typography variant="body" className="text-content-secondary">
              {gift.message || t('gift.defaultMessage')}
            </Typography>
            <Typography variant="display" as="h1" id="gift-title" className="mt-2">
              {t('gift.credits', { credits: gift.credits })}
            </Typography>
            <Typography variant="body" className="mt-2">
              {t('gift.validity', { days: gift.validityDays })}
            </Typography>
            <Typography variant="label" as="p" className="text-content-secondary mt-1">
              {t('gift.linkExpires', { at: gift.linkExpiresAt })}
            </Typography>
            <RedeemVoucher token={token} signedIn={signedIn} />
          </>
        )}
        {!isPending && terminal && (
          <>
            <Typography variant="display" as="h1" id="gift-title">
              {t(
                notFound
                  ? 'gift.notFound'
                  : UNAVAILABLE_HEADING[
                      gift?.state === 'redeemable' || !gift ? 'unknown' : gift.state
                    ],
              )}
            </Typography>
            <Typography variant="body" className="text-content-secondary mt-3">
              {t('gift.unavailableBody')}
            </Typography>
            <Link to="/" className={buttonStyles({ variant: 'secondary', className: 'mt-6' })}>
              {t('gift.home')}
            </Link>
          </>
        )}
        {!isPending && failure && !notFound && (
          <Notice tone="danger" role="alert">
            <Typography variant="body" as="h1" id="gift-title">
              <AppFailureMessage failure={failure} />
            </Typography>
          </Notice>
        )}
      </section>
    </main>
  )
}
