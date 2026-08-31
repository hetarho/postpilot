import { useTranslation } from 'react-i18next'
import type { RecommendationSet } from '@/entities/model-catalog'
import { useApplyRecommendation } from '@/entities/model-catalog'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

export function ApplyRecommendation({ recommendation }: { recommendation: RecommendationSet }) {
  const { t } = useTranslation('models')
  const mutation = useApplyRecommendation()
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
        pending={mutation.isPending}
        onClick={() => {
          void mutation.apply(recommendation.id).catch(() => {
            // The mutation state carries the structured failure rendered below.
          })
        }}
      >
        {t('recommendation.apply')}
      </Button>
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
