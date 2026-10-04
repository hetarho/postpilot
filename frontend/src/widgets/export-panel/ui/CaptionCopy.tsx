import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { CopyFallbackElement } from '@/shared/lib'
import { FieldLabel, TextField, Typography } from '@/shared/ui'
import { useCaptionReveal } from '../model/useCaptionReveal'

export interface CaptionCopyProps {
  caption: string
  captionStatus: string
  captionFellBack: boolean
  onCopyCaption: () => void
  registerCaptionField: (element: CopyFallbackElement | null) => void
}

/** A caption as its own copy control (EXPORT-24): a single photo's, or the one caption of a photo
 *  group (EXPORT-26). A block with no caption renders nothing at all. */
export function CaptionCopy({
  caption,
  ariaLabel,
  captionStatus,
  captionFellBack,
  onCopyCaption,
  registerCaptionField,
}: CaptionCopyProps & { ariaLabel: string }) {
  const { t } = useTranslation('posts')
  const captionFieldId = useId()
  // The same reveal the Naver body does, one caption down.
  const { controlRef: captionButtonRef, fieldRef: captionFieldRef } =
    useCaptionReveal(captionFellBack)
  // The caption IS the control, the way the photo above is: it is the full width of the column,
  // it is the text the user is looking at, and a 44px button beside it would be a fraction of
  // that reach. A block with no caption renders nothing here at all — no control, no status line
  // (EXPORT-24).
  if (!caption) return null
  return (
    <div className="mt-2">
      <button
        type="button"
        ref={captionButtonRef}
        aria-label={ariaLabel}
        onClick={onCopyCaption}
        className="block w-full cursor-pointer rounded-lg text-left active:brightness-90"
      >
        <Typography variant="label" as="span" className="block break-words">
          {caption}
        </Typography>
      </button>
      {/* Revealed only by a refused copy, exactly like the Naver body's raw field: there is
              nothing to select until there is something to select. */}
      {captionFellBack && (
        <>
          <FieldLabel htmlFor={captionFieldId} className="sr-only">
            {t('export.captionField')}
          </FieldLabel>
          <TextField
            id={captionFieldId}
            ref={(element) => {
              captionFieldRef.current = element
              registerCaptionField(element)
            }}
            value={caption}
            readOnly
            className="mt-2 w-full"
          />
        </>
      )}
      <Typography
        variant="meta"
        as="p"
        role="status"
        className="text-content-tertiary mt-1 min-h-4 break-words"
      >
        {captionStatus}
      </Typography>
    </div>
  )
}
