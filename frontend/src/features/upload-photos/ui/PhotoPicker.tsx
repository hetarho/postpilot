import { useId, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { UPLOAD_ALLOWED_EXTENSIONS } from '@/shared/config'
import { buttonStyles } from '@/shared/ui'

// `image/*` alongside the extensions: a phone picker keys off MIME types and would
// otherwise hide the camera roll on some devices; the extension gate runs afterwards
// regardless (model/filter.ts).
const ACCEPT = [...UPLOAD_ALLOWED_EXTENSIONS.map((extension) => `.${extension}`), 'image/*'].join(
  ',',
)

// Named peers (`peer/gallery`), spelled out rather than built: `peer-focus-visible:` is a general
// sibling selector, so with two inputs in the row an unnamed peer would ring both labels from
// either input — and Tailwind only emits a class it can read whole in the source.
const GALLERY_FOCUS =
  'peer-focus-visible/gallery:outline-focus-ring peer-focus-visible/gallery:outline-2 peer-focus-visible/gallery:outline-offset-2'
const CAMERA_FOCUS =
  'peer-focus-visible/camera:outline-focus-ring peer-focus-visible/camera:outline-2 peer-focus-visible/camera:outline-offset-2'

export function PhotoPicker({
  onFiles,
  disabled,
}: {
  onFiles: (files: File[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation('posts')
  const galleryId = useId()
  const cameraId = useId()
  const pick = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    // Cleared so picking the same photo again fires a change event.
    event.target.value = ''
    onFiles(files)
  }

  return (
    <>
      <input
        id={galleryId}
        type="file"
        multiple
        accept={ACCEPT}
        disabled={disabled}
        className="peer/gallery sr-only"
        onChange={pick}
      />
      <label
        htmlFor={galleryId}
        aria-disabled={disabled || undefined}
        className={buttonStyles({
          variant: 'secondary',
          className: disabled ? undefined : GALLERY_FOCUS,
        })}
      >
        {t('upload.add')}
      </label>
      {/* A second input, because `capture` belongs to the control and not to the pick: an
          image-only `accept` sends Android Chrome to the system photo picker, which offers no way
          out to the camera at all, so the product's opening move — photograph the thing, then
          write about it — cannot start from the camera. Putting `capture` on the input above
          would instead make it camera-only and kill multi-select. Both feed the same handler, so
          the filter and the batch see one kind of pick. */}
      <input
        id={cameraId}
        type="file"
        accept={ACCEPT}
        capture="environment"
        disabled={disabled}
        className="peer/camera sr-only"
        onChange={pick}
      />
      <label
        htmlFor={cameraId}
        aria-disabled={disabled || undefined}
        className={buttonStyles({
          variant: 'ghost',
          className: disabled ? undefined : CAMERA_FOCUS,
        })}
      >
        {t('upload.camera')}
      </label>
    </>
  )
}
