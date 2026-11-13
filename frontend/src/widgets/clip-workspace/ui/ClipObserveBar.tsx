import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipPreparationScreen } from '@/features/edit-clip-project'
import { Button, StepBarActions, StepBarBack, StepBarReason } from '@/shared/ui'

/** Why ②'s actions are held: with a route back to ①'s screen or a retry when the owner can fix
 *  it, or only the reason while other work finishes. */
export type ClipObserveCorrection =
  | { screen: ClipPreparationScreen; label: string }
  | { retry: () => void; label: string }
  | { label: string }

/** The body of ②'s step bar (CLIP-40): what holds or refused the run, then 이전 back to ① and the
 *  separately quoted `스토리라인 먼저` and `바로 만들기` as one row at every width. A reason stands
 *  above its control, named by it through `reasonId`, so the keyboard never hides it (THEME-71). */
export function ClipObserveBar({
  actions,
  refusal,
  correction,
  reasonId,
  onBack,
  onRoute,
}: {
  actions: ReactNode
  refusal?: ReactNode
  correction?: ClipObserveCorrection
  reasonId: string
  onBack: () => void
  /** Brings back ①'s screen the correction names, with its invalid field focused. */
  onRoute: (screen: ClipPreparationScreen) => void
}) {
  const { t } = useTranslation('clips')
  return (
    <>
      {refusal}
      {correction && (
        <div className="flex flex-wrap items-center gap-2">
          <StepBarReason id={reasonId}>{correction.label}</StepBarReason>
          {('screen' in correction || 'retry' in correction) && (
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                'screen' in correction ? onRoute(correction.screen) : correction.retry()
              }
            >
              {t('guide.fix')}
            </Button>
          )}
        </div>
      )}
      {/* Two quoted actions beside the way back can outgrow a 320px row: wrapping gives the
          committing one a row of its own rather than a label broken per syllable. */}
      <StepBarActions
        back={<StepBarBack label={t('observeStep.back')} onClick={onBack} />}
        className="flex-wrap"
      >
        {actions}
      </StepBarActions>
    </>
  )
}
