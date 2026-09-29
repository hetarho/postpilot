import { useTranslation } from 'react-i18next'
import type { CatalogModel, ModelRef, RecommendationSet, StageName } from '@/entities/model-catalog'
import {
  modelChoiceIssue,
  refKey,
  sameRef,
  useApplyRecommendation,
  useModels,
} from '@/entities/model-catalog'
import { AppFailureMessage, Button, Notice, Typography } from '@/shared/ui'

export function ApplyRecommendation({ recommendation }: { recommendation: RecommendationSet }) {
  const { t } = useTranslation('models')
  const mutation = useApplyRecommendation()
  const { models, isPending, isError } = useModels()
  // A set is applied whole: the server refuses all seven refs if any one is above the tier, so
  // offering the button would only produce a refusal the user cannot act on from here.
  const blocked = isPending || isError ? [] : unavailableRefs(recommendation, models, t('vanished'))
  return (
    // No card: this is the whole content of a page section, and THEME-13 excludes a section from the
    // card contract. On a 360px phone its padding cost 32px of a 328px column in the one region
    // THEME-8 says content should be largest, and pushed everything below it further from the thumb.
    <div>
      <Typography variant="label" as="p" className="text-content-primary">
        {recommendation.label}
      </Typography>
      {/* `break-words`: the set id is a server-supplied slug (THEME-21). */}
      <Typography variant="body" as="p" className="text-content-secondary mt-1 break-words">
        <Typography variant="meta" as="span" mono>
          {recommendation.id}
        </Typography>{' '}
        · {t('recommendation.description')}
      </Typography>
      <Button
        variant="secondary"
        className="mt-4 w-full sm:w-auto"
        disabled={isPending || isError || blocked.length > 0}
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
          {t('recommendation.unavailable', {
            models: blocked.map(({ ref, reason }) => `${refKey(ref)} (${reason})`).join(', '),
          })}
        </Notice>
      )}
      {isError && (
        <Notice tone="danger" role="alert" className="mt-2">
          {t('selectField.loadFailed')}
        </Notice>
      )}
      {/* Everything this action rewrites — the three active models and the observe and write A/B
          selects — is 400–900px further down the page, off-screen on any phone. Without a
          confirmation beside the button the only visible result of a successful apply is the
          spinner going away, which reads exactly like a failure (THEME-24). */}
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

/** Every ref in the set the calling account may not run, in set order and without repeats: the
 *  seven a set carries, analyze naming its active model alone (MODEL-23). */
function unavailableRefs(
  recommendation: RecommendationSet,
  models: readonly CatalogModel[],
  missingReason: string,
): { ref: ModelRef; reason: string }[] {
  const locked: { ref: ModelRef; reason: string }[] = []
  for (const selection of recommendation.selections) {
    const stage: StageName = selection.stage
    const refs = [selection.active, selection.candidateA, selection.candidateB]
    for (const ref of refs.filter((value) => value !== undefined)) {
      const model = models.find((candidate) => sameRef(candidate.ref, ref))
      const reason = model ? modelChoiceIssue(model, stage) : missingReason
      if (!reason) continue
      if (locked.some((existing) => sameRef(existing.ref, ref))) continue
      locked.push({ ref, reason })
    }
  }
  return locked
}
