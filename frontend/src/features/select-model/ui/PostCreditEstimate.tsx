import { useTranslation } from 'react-i18next'
import {
  pairPostFigure,
  postCreditLabel,
  sameRef,
  useModels,
  useStageSelection,
} from '@/entities/model-catalog'
import { useMyPlan, postsAffordable } from '@/entities/plan'
import { Typography } from '@/shared/ui'

/** What one post costs on the selected pair, and how far the balance goes at that rate.
 *
 *  Both numbers come from the server: each model's per-stage figure from ListModels (recent
 *  real usage, or the catalog-based estimate labelled 예상, QUOTA-64) and the balance from
 *  GetMyPlan. This component only adds the two stage figures and divides, so a user is never
 *  shown a rate the server did not publish.
 *
 *  `photoCount` is the post's, where there is one: a post with no photo never observes, so
 *  the observe figure counts only when it has one. Without it (the model settings page) the
 *  figure is a post with photos. */
export function PostCreditEstimate({
  className,
  photoCount,
}: {
  className?: string
  photoCount?: number
}) {
  const { t } = useTranslation('plans')
  const { models } = useModels()
  const observe = useStageSelection('observe')
  const write = useStageSelection('write')
  const { myPlan, isPending: planPending } = useMyPlan()

  if (observe.isPending || write.isPending || planPending || !myPlan) return null
  const find = (selected: typeof write.selected) =>
    selected ? models.find((model) => sameRef(model.ref, selected)) : undefined
  const figure = pairPostFigure({
    observe: find(observe.selected),
    write: find(write.selected),
    withPhotos: photoCount === undefined || photoCount > 0,
  })
  // No write model, a paid stage with no figure, or a pair of free models: there is nothing
  // to price, and a zero would read as "free".
  if (!figure) return null

  // The operator is exempt from credits, so a posts count would be meaningless; the figure
  // itself still says what the pair costs everyone else.
  if (myPlan.balance.unlimited) {
    return (
      <div className={className}>
        <Typography variant="body" role="status">
          {postCreditLabel(figure)}
        </Typography>
      </div>
    )
  }

  const posts = postsAffordable(myPlan.balance.credits, figure.credits)
  return (
    <div className={className}>
      <Typography variant="body" role="status">
        {posts > 0 ? t('estimate.posts', { count: posts }) : t('estimate.none')}
      </Typography>
      <Typography variant="meta" className="text-content-tertiary mt-1 block">
        {postCreditLabel(figure)} {t('estimate.caveat')}
      </Typography>
    </div>
  )
}
