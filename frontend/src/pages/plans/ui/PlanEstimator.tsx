import { useTranslation } from 'react-i18next'
import { CLIP_ESTIMATE_BOUNDS, PLAN_ESTIMATE_BOUNDS } from '../config'
import { Slider, Typography } from '@/shared/ui'
import type { ClipEstimateInput, EstimateInput, EstimateKind } from '../model/estimate-input'

/** Conditions for ONE finished item. Capacity comparisons stay in the plan cards. */
export function PlanEstimator({
  kind,
  input,
  clipInput,
  sourceSeconds,
  onInputChange,
  onClipInputChange,
}: {
  kind: EstimateKind
  input: EstimateInput
  clipInput: ClipEstimateInput
  sourceSeconds: number
  onInputChange: (input: EstimateInput) => void
  onClipInputChange: (input: ClipEstimateInput) => void
}) {
  const { t } = useTranslation('plans')
  return (
    <div className="grid gap-6">
      <Typography variant="body" className="text-content-secondary">
        {t(kind === 'blog' ? 'estimator.description' : 'estimator.clipDescription')}
      </Typography>
      {kind === 'blog' ? (
        <>
          {(['chars', 'photos', 'videos'] as const).map((field) => (
            <Slider
              key={field}
              label={t(`estimator.${field}`)}
              value={input[field]}
              {...PLAN_ESTIMATE_BOUNDS[field]}
              valueText={t(`estimator.${field}Value`, { count: input[field] })}
              onChange={(value) => onInputChange({ ...input, [field]: value })}
            />
          ))}
        </>
      ) : (
        <>
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
        </>
      )}
    </div>
  )
}
