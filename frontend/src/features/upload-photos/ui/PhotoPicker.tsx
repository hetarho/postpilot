import { useId } from 'react'
import { UPLOAD_ALLOWED_EXTENSIONS } from '@/shared/config'
import { buttonStyles } from '@/shared/ui'

// `image/*` alongside the extensions: a phone picker keys off MIME types and would
// otherwise hide the camera roll on some devices; the extension gate runs afterwards
// regardless (model/filter.ts).
const ACCEPT = [...UPLOAD_ALLOWED_EXTENSIONS.map((extension) => `.${extension}`), 'image/*'].join(
  ',',
)

export function PhotoPicker({
  onFiles,
  disabled,
}: {
  onFiles: (files: File[]) => void
  disabled?: boolean
}) {
  const inputId = useId()
  return (
    <>
      <input
        id={inputId}
        type="file"
        multiple
        accept={ACCEPT}
        disabled={disabled}
        className="peer sr-only"
        onChange={(event) => {
          const files = Array.from(event.target.files ?? [])
          // Cleared so picking the same photo again fires a change event.
          event.target.value = ''
          onFiles(files)
        }}
      />
      <label
        htmlFor={inputId}
        aria-disabled={disabled || undefined}
        className={buttonStyles({
          variant: 'secondary',
          className: disabled
            ? undefined
            : 'peer-focus-visible:outline-focus-ring cursor-pointer peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2',
        })}
      >
        사진 추가
      </label>
    </>
  )
}
