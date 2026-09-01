import { useState, type DragEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Typography } from '@/shared/ui'

interface PhotoDropZoneProps {
  onFiles: (files: File[]) => void
  disabled?: boolean
  children: ReactNode
}

// A drag carrying files announces itself as `Files`; text or link drags from elsewhere on the
// page are left alone so the section never lights up for something it cannot take.
const carriesFiles = (event: DragEvent) => Array.from(event.dataTransfer.types).includes('Files')

/**
 * Turns whatever it wraps into a drop target for photos — the desktop counterpart of the picker
 * buttons. The picker stays the only keyboard/screen-reader route; the zone adds nothing to the
 * accessibility tree except a cue while a file drag is over it.
 */
export function PhotoDropZone({ onFiles, disabled, children }: PhotoDropZoneProps) {
  const { t } = useTranslation('posts')
  // Counted rather than toggled: `dragleave` fires for every child the pointer crosses, and only
  // the balance says whether the pointer has left the zone itself.
  const [depth, setDepth] = useState(0)
  const active = depth > 0 && !disabled

  const enter = (event: DragEvent<HTMLDivElement>) => {
    if (disabled || !carriesFiles(event)) return
    event.preventDefault()
    setDepth((current) => current + 1)
  }
  const over = (event: DragEvent<HTMLDivElement>) => {
    if (disabled || !carriesFiles(event)) return
    // Without this the browser refuses the drop (and navigates to the image on release).
    event.preventDefault()
    event.dataTransfer.dropEffect = 'copy'
  }
  const leave = (event: DragEvent<HTMLDivElement>) => {
    if (disabled || !carriesFiles(event)) return
    setDepth((current) => Math.max(0, current - 1))
  }
  const drop = (event: DragEvent<HTMLDivElement>) => {
    if (disabled || !carriesFiles(event)) return
    event.preventDefault()
    setDepth(0)
    const files = Array.from(event.dataTransfer.files)
    if (files.length > 0) onFiles(files)
  }

  return (
    <div
      data-slot="photo-drop-zone"
      data-active={active || undefined}
      onDragEnter={enter}
      onDragOver={over}
      onDragLeave={leave}
      onDrop={drop}
      className="relative"
    >
      {children}
      {active && (
        <Typography
          variant="body"
          as="div"
          aria-hidden
          className="bg-notice-info-bg/80 text-notice-info-fg outline-focus-ring pointer-events-none absolute -inset-2 flex items-center justify-center rounded-lg outline-2 outline-dashed"
        >
          {t('upload.drop')}
        </Typography>
      )}
    </div>
  )
}
