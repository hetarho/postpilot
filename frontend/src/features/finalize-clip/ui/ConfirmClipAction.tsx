import { useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { AppFailureMessage, Notice, StepBarCta, StepBarReason } from '@/shared/ui'
import type { ClipConfirmationRefusal } from '../model/confirmation-refusal'
import type { useConfirmClip } from '../model/useConfirmClip'

/** ②'s big action, 확정하기, in the step bar's row (CLIP-40). Like the post's 확정하기 (POST-56) it
 *  runs at once, and its reason and failure render ABOVE the row through `children`, across the
 *  whole bar, so the software keyboard may hide the control but never why it is held
 *  (THEME-31). */
export function ConfirmClipAction({
  action,
  refusal,
  fix,
  children,
}: {
  action: ReturnType<typeof useConfirmClip>
  refusal?: ClipConfirmationRefusal
  /** The way to resolve a refusal, beside its reason — narration's script, for one. */
  fix?: ReactNode
  /** Places the control in the bar's row. */
  children: (control: ReactNode) => ReactNode
}) {
  const { t } = useTranslation('clips')
  const reasonId = useId()
  return (
    <>
      {refusal && (
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <StepBarReason id={reasonId}>{t(`confirmation.refusal.${refusal}`)}</StepBarReason>
          {fix}
        </div>
      )}
      {action.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={action.failure} />
        </Notice>
      )}
      {children(
        <StepBarCta
          pending={action.pending}
          disabled={!!refusal}
          aria-describedby={refusal ? reasonId : undefined}
          onClick={() => void action.confirm()}
        >
          {t('confirmation.action')}
        </StepBarCta>,
      )}
    </>
  )
}
