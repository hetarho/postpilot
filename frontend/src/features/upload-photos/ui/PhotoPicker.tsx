import { useId } from 'react'
import { clsx } from 'clsx'
import { UPLOAD_ALLOWED_EXTENSIONS } from '@/shared/config'

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
      <label
        htmlFor={inputId}
        className={clsx(
          'rounded-md bg-surface-raised px-2.5 py-1 text-xs text-text-muted',
          disabled ? 'opacity-50' : 'cursor-pointer hover:text-text',
        )}
      >
        사진 추가
      </label>
      <input
        id={inputId}
        type="file"
        multiple
        accept={ACCEPT}
        disabled={disabled}
        className="sr-only"
        onChange={(event) => {
          const files = Array.from(event.target.files ?? [])
          // Cleared so picking the same photo again fires a change event.
          event.target.value = ''
          onFiles(files)
        }}
      />
    </>
  )
}
