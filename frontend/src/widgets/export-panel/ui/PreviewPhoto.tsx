import { useTranslation } from 'react-i18next'
import { CaptionCopy, type CaptionCopyProps } from './CaptionCopy'
import { PhotoCopy, type PhotoCopyProps } from './PhotoCopy'

/** One inline photo of the Naver preview: the pixels at their natural width, the photo ITSELF as
 *  the copy control, that photo's own status line, and — under it — the caption as a second
 *  control of its own (EXPORT-24), because the marker in the pasted text carries neither.
 *
 *  It reports its own state under its own photo rather than in one shared line, because "which
 *  photo failed" is the only useful part of the message (THEME-24). */
export function PreviewPhoto({
  file,
  alt,
  caption,
  image,
  marker,
  copied,
  failure,
  captionStatus,
  captionFellBack,
  onCopy,
  onCopyCaption,
  registerCaptionField,
  onStale,
}: PhotoCopyProps & CaptionCopyProps) {
  const { t } = useTranslation('posts')
  return (
    <div className="py-2">
      <PhotoCopy
        file={file}
        alt={alt}
        image={image}
        marker={marker}
        copied={copied}
        failure={failure}
        onCopy={onCopy}
        onStale={onStale}
      />
      <CaptionCopy
        caption={caption}
        ariaLabel={t('export.captionCopyAria', { number: marker + 1 })}
        captionStatus={captionStatus}
        captionFellBack={captionFellBack}
        onCopyCaption={onCopyCaption}
        registerCaptionField={registerCaptionField}
      />
    </div>
  )
}
