import { useTranslation } from 'react-i18next'
import { PLANS, planLabel, type PlanName } from '@/entities/plan'
import type { CatalogModel, ModelRef, RecommendationSet } from '@/entities/model-catalog'
import { refKey, sameRef, useApplyRecommendation, useModels } from '@/entities/model-catalog'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

export function ApplyRecommendation({ recommendation }: { recommendation: RecommendationSet }) {
  const { t } = useTranslation('models')
  const mutation = useApplyRecommendation()
  const { models } = useModels()
  // A set is applied whole: the server refuses all nine refs if any one is above the tier, so
  // offering the button would only produce a refusal the user cannot act on from here.
  const blocked = lockedRefs(recommendation, models)
  return (
    // No card: this is the whole content of a page section, and §1.4 excludes a section from the
    // card contract. On a 360px phone its padding cost 32px of a 328px column in the one region
    // §0 says content should be largest, and pushed everything below it further from the thumb.
    <div>
      <p className="text-sm font-medium">{recommendation.label}</p>
      {/* `break-words`: the set id is a server-supplied slug (§3.2). */}
      <p className="text-content-tertiary mt-1 text-xs break-words">
        {recommendation.id} · {t('recommendation.description')}
      </p>
      <Button
        variant="secondary"
        className="mt-4 w-full sm:w-auto"
        disabled={blocked.length > 0}
        pending={mutation.isPending}
        onClick={() => {
          void mutation.apply(recommendation.id).catch(() => {
            // The mutation state carries the structured failure rendered below.
          })
        }}
      >
        {t('recommendation.apply')}
      </Button>
      {blocked.length > 0 && (
        <Notice tone="info" role="status" className="mt-2">
          {t('recommendation.locked', {
            models: blocked.map(refKey).join(', '),
            plan: planLabel(highestFloor(blocked, models)),
          })}
        </Notice>
      )}
      {/* Everything this action rewrites — the active model and the A/B selects for all three
          stages — is 400–900px further down the page, off-screen on any phone. Without a
          confirmation beside the button the only visible result of a successful apply is the
          spinner going away, which reads exactly like a failure (§4.3). */}
      {mutation.isSuccess && (
        <Notice tone="success" role="status" className="mt-2">
          {t('recommendation.applied')}
        </Notice>
      )}
      {mutation.failure && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={mutation.failure} />
        </Notice>
      )}
    </div>
  )
}

/** Every ref in the set the calling account may not run, in set order and without repeats. */
function lockedRefs(
  recommendation: RecommendationSet,
  models: readonly CatalogModel[],
): ModelRef[] {
  const locked: ModelRef[] = []
  for (const selection of recommendation.selections) {
    for (const ref of [selection.active, selection.candidateA, selection.candidateB]) {
      const model = models.find((candidate) => sameRef(candidate.ref, ref))
      if (!model?.locked) continue
      if (locked.some((existing) => sameRef(existing, ref))) continue
      locked.push(ref)
    }
  }
  return locked
}

/** The one upgrade that unlocks the whole set — the highest floor among what it blocked.
 *  Ranked through PLANS so a tier added to the ladder is ordered without editing this. */
function highestFloor(
  locked: readonly ModelRef[],
  models: readonly CatalogModel[],
): PlanName | undefined {
  return locked
    .flatMap((ref) => {
      const model = models.find((candidate) => sameRef(candidate.ref, ref))
      return model?.minPlan ? [model.minPlan] : []
    })
    .reduce<PlanName | undefined>(
      (highest, floor) =>
        highest === undefined || PLANS.indexOf(floor) > PLANS.indexOf(highest) ? floor : highest,
      undefined,
    )
}
