import { useTranslation } from 'react-i18next'
import { CLIP_ESTIMATE_BOUNDS } from '../config'
import { Slider, Typography } from '@/shared/ui'
import type { ClipEstimateInput } from '../model/estimate-input'

/** Conditions for ONE finished clip. A post needs none: its figure comes from recent real usage
 *  (QUOTA-64). Capacity comparisons stay in the plan cards. */
export function PlanEstimator({
  clipInput,
  sourceSeconds,
  onClipInputChange,
}: {
  clipInput: ClipEstimateInput
  sourceSeconds: number
  onClipInputChange: (input: ClipEstimateInput) => void
}) {
  const { t } = useTranslation('plans')
  return (
    <div className="grid gap-6">
      <Typography variant="body" className="text-content-secondary">
        {t('estimator.clipDescription')}
      </Typography>
      {(['sources', 'seconds'] as const).map((field) => (
        <Slider
          key={field}
          label={t(`estimator.${field}`)}
          value={clipInput[field]}
          {...CLIP_ESTIMATE_BOUNDS[field]}
          valueText={t(`estimator.${field}Value`, { count: clipInput[field] })}
          onChange={(value) => onClipInputChange({ ...clipInput, [field]: value })}
        />
      ))}
      {sourceSeconds > 0 && (
        <Typography variant="meta" className="text-content-secondary">
          {t('estimator.sourceAssumption', { seconds: sourceSeconds })}
        </Typography>
      )}
    </div>
  )
}
