import { useEffect } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { useMyPlanQueryKey } from '@/entities/plan'
import { useGiftQueryKey, useRedeemVoucher } from '@/entities/voucher'
import { SIGNED_IN_HOME } from '@/shared/lib'
import { AppFailureMessage, Button, Notice, buttonStyles } from '@/shared/ui'
import { clearPendingGift, savePendingGift } from '../lib/pending-gift'

/** The one action a redeemable gift link offers (GIFT-8, GIFT-9).
 *
 *  Signed in, it is a single 받기 that never fires on its own. Anonymous, it is the way in —
 *  log in or sign up with this page as the destination — and the link is remembered so the
 *  email-verification detour still comes back here. */
export function RedeemVoucher({ token, signedIn }: { token: string; signedIn: boolean }) {
  const { t } = useTranslation('plans')
  const queryClient = useQueryClient()
  const planKey = useMyPlanQueryKey()
  const giftKey = useGiftQueryKey(token)
  const redeem = useRedeemVoucher({
    onRedeemed: () => {
      clearPendingGift()
      void queryClient.invalidateQueries({ queryKey: planKey })
    },
    // A link that was spent, lapsed or revoked meanwhile: the page re-reads it and shows that.
    onRefused: (failure) => {
      if (failure.reason.startsWith('VOUCHER_')) {
        void queryClient.invalidateQueries({ queryKey: giftKey })
      }
    },
  })

  useEffect(() => {
    if (!signedIn) savePendingGift(token)
  }, [signedIn, token])

  if (redeem.redemption) {
    return (
      <div className="mt-6 grid gap-4">
        <Notice tone="success" role="status">
          {t('redeemVoucher.redeemed', {
            credits: redeem.redemption.credits,
            at: redeem.redemption.creditsExpireAt,
          })}
        </Notice>
        <Link to={SIGNED_IN_HOME} className={buttonStyles({ variant: 'cta' })}>
          {t('redeemVoucher.start')}
        </Link>
      </div>
    )
  }

  if (!signedIn) {
    const redirect = `/gift/${encodeURIComponent(token)}`
    return (
      <div className="mt-6 grid gap-3">
        <Link to="/login" search={{ redirect }} className={buttonStyles({ variant: 'cta' })}>
          {t('redeemVoucher.signIn')}
        </Link>
        <Link to="/signup" search={{ redirect }} className={buttonStyles({ variant: 'secondary' })}>
          {t('redeemVoucher.signUp')}
        </Link>
      </div>
    )
  }

  return (
    <div className="mt-6 grid gap-3">
      {redeem.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={redeem.failure} />
        </Notice>
      )}
      <Button variant="cta" pending={redeem.isPending} onClick={() => redeem.redeem(token)}>
        {t('redeemVoucher.redeem')}
      </Button>
    </div>
  )
}
