import type { ComponentProps, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ConfirmClipAction,
  type ClipConfirmationRefusal,
  type useConfirmClip,
} from '@/features/finalize-clip'
import { ClipRevisionRequest } from '@/features/revise-clip'
import { StepBarActions, StepBarBack } from '@/shared/ui'

/** The body of ②'s step bar, the post's 글 다듬기 dock in the clip (CLIP-40 →POST-56): the
 *  revision request with its scene chip, then the row every step ends on — 이전 back to 영상
 *  생성 and 확정하기 as the big action, which confirms at once (CLIP-75).
 *
 *  It is a WIDGET because it composes two sibling features — `revise-clip` and `finalize-clip` —
 *  and a feature may not import a sibling (ARCH-13). 확정하기's reason and failure stand above
 *  the composer, across the whole bar, so the keyboard may hide the row but never why its action
 *  is held (THEME-31). */
export function ClipRefineDock({
  request,
  confirm,
  refusal,
  fix,
  onBack,
}: {
  request: ComponentProps<typeof ClipRevisionRequest>
  confirm: ReturnType<typeof useConfirmClip>
  refusal?: ClipConfirmationRefusal
  /** The way to resolve the refusal beside its reason. */
  fix?: ReactNode
  onBack: () => void
}) {
  const { t } = useTranslation('clips')
  return (
    <ConfirmClipAction action={confirm} refusal={refusal} fix={fix}>
      {(control) => (
        <>
          <ClipRevisionRequest {...request} />
          <StepBarActions back={<StepBarBack label={t('refineDock.back')} onClick={onBack} />}>
            {control}
          </StepBarActions>
        </>
      )}
    </ConfirmClipAction>
  )
}
