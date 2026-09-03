import { useTranslation } from 'react-i18next'
import { useSelections } from '@/entities/model-catalog'
import { useMyPlan, postsAffordable } from '@/entities/plan'
import { Typography } from '@/shared/ui'

/** What the current stage pair costs, and how far the balance goes at that rate.
 *
 *  Both numbers come from the server: the per-post estimate from the same estimator the
 *  gate applies at start, the balance from GetMyPlan. This component only divides them, so
 *  a user is never quoted a rate the charge could disagree with.
 *
 *  It is an estimate and says so: the real cost moves with photo count and post length, and
 *  a figure presented as exact would be wrong the first time someone attached ten photos. */
export function PostCreditEstimate({ className }: { className?: string }) {
  const { t } = useTranslation('plans')
  const { estimatedPostCredits, isPending: selectionsPending } = useSelections()
  const { myPlan, isPending: planPending } = useMyPlan()

  if (selectionsPending || planPending || !myPlan) return null
  // No pair chosen yet: there is nothing to price, and a zero would read as "free".
  if (estimatedPostCredits <= 0) return null
  if (myPlan.balance.unlimited) return null

  const posts = postsAffordable(myPlan.balance.credits, estimatedPostCredits)

  return (
    <div className={className}>
      <Typography variant="body" role="status">
        {posts > 0 ? t('estimate.posts', { count: posts }) : t('estimate.none')}
      </Typography>
      <Typography variant="meta" className="text-content-tertiary mt-1 block">
        {t('estimate.perPost', { credits: estimatedPostCredits })} {t('estimate.caveat')}
      </Typography>
    </div>
  )
}
