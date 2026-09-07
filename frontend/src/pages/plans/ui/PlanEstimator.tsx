import { useTranslation } from 'react-i18next'
import type { EstimatorCombo, EstimatorComboName } from '@/entities/plan'
import { PLAN_ESTIMATE_BOUNDS } from '@/shared/config'
import { SegmentedControl, Slider, Typography } from '@/shared/ui'
import type { EstimateInput } from '../model/estimate-input'

/** The three sliders and the combo switch above the rungs.
 *
 *  Every rung's post count is derived from these three numbers and the selected combo's
 *  published rates, so the control belongs above the list it changes rather than inside any
 *  one card.
 *
 *  Nothing here asks the server: the rates arrived with GetMyPlan and the page multiplies
 *  (QUOTA-40), which is what lets a slider answer while it is being dragged. */
export function PlanEstimator({
  combos,
  combo,
  input,
  onComboChange,
  onInputChange,
}: {
  combos: readonly EstimatorCombo[]
  combo: EstimatorComboName | undefined
  input: EstimateInput
  onComboChange: (combo: EstimatorComboName) => void
  onInputChange: (input: EstimateInput) => void
}) {
  const { t } = useTranslation('plans')

  return (
    <div className="bg-surface-raised mt-8 rounded-lg p-4">
      <Typography variant="title" as="h2">
        {t('estimator.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-1 block">
        {t('estimator.description')}
      </Typography>

      <div className="mt-4 grid gap-4 md:grid-cols-3">
        <Slider
          label={t('estimator.chars')}
          value={input.chars}
          min={PLAN_ESTIMATE_BOUNDS.chars.min}
          max={PLAN_ESTIMATE_BOUNDS.chars.max}
          step={PLAN_ESTIMATE_BOUNDS.chars.step}
          valueText={t('estimator.charsValue', { count: input.chars })}
          onChange={(chars) => onInputChange({ ...input, chars })}
        />
        <Slider
          label={t('estimator.photos')}
          value={input.photos}
          min={PLAN_ESTIMATE_BOUNDS.photos.min}
          max={PLAN_ESTIMATE_BOUNDS.photos.max}
          step={PLAN_ESTIMATE_BOUNDS.photos.step}
          valueText={t('estimator.photosValue', { count: input.photos })}
          onChange={(photos) => onInputChange({ ...input, photos })}
        />
        <Slider
          label={t('estimator.videos')}
          value={input.videos}
          min={PLAN_ESTIMATE_BOUNDS.videos.min}
          max={PLAN_ESTIMATE_BOUNDS.videos.max}
          step={PLAN_ESTIMATE_BOUNDS.videos.step}
          valueText={t('estimator.videosValue', { count: input.videos })}
          onChange={(videos) => onInputChange({ ...input, videos })}
        />
      </div>

      {combo !== undefined && combos.length > 0 ? (
        <div className="mt-4">
          <Typography variant="label" as="p" className="mb-1 block">
            {t('estimator.combo')}
          </Typography>
          <SegmentedControl
            ariaLabel={t('estimator.combo')}
            value={combo}
            options={combos.map((assigned) => ({
              value: assigned.combo,
              label: t(`estimator.combos.${assigned.combo}`),
            }))}
            onChange={onComboChange}
          />
        </div>
      ) : (
        // No combo assigned is an operator state, not a failure: the rungs still say what
        // they grant, and nothing pretends to know what that buys.
        <Typography variant="meta" role="status" className="text-content-tertiary mt-4 block">
          {t('estimator.unset')}
        </Typography>
      )}
    </div>
  )
}
