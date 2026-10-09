import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { Button, RotatedImage, Sheet, Typography } from '@/shared/ui'
import type { ViewableAttachment } from '../model/attachments'

/** One attachment, large (POST-100): a photo fitted to the view, a clip playing with its own
 *  controls, and the previous and next attachment of the surface one press (or one arrow key) away.
 *  It only shows — rotating, deleting, moving and placing stay on the tiles — so nothing here saves,
 *  and a read-only post opens it as well.
 *
 *  `files` is the walk (①'s strip order, or `storylineViewOrder` in the storyline space) and
 *  `viewing` the one on screen; `null` closes. */
export function AttachmentViewer({
  files,
  attachments,
  viewing,
  onView,
  onClose,
}: {
  files: readonly string[]
  attachments: ReadonlyMap<string, ViewableAttachment>
  viewing: string | null
  onView: (file: string) => void
  onClose: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  const headingId = useId()
  const index = viewing ? files.indexOf(viewing) : -1
  const open = index >= 0
  // The sheet outlives `open` by its exit animation; what it shows on the way out is the last
  // attachment it showed, not an empty panel.
  const [shown, setShown] = useState(viewing)
  if (open && viewing !== shown) setShown(viewing)
  const file = open ? viewing : shown
  const attachment = file ? attachments.get(file) : undefined

  const previous = useRef<HTMLButtonElement>(null)
  const next = useRef<HTMLButtonElement>(null)
  // A step onto either end disables the button just pressed; the keyboard then belongs on the one
  // that can still move, not on the body behind the sheet.
  const refocus = useRef<'previous' | 'next' | null>(null)
  const go = (step: -1 | 1) => {
    const to = files[index + step]
    if (!to) return
    const landsOnEnd = index + step === 0 || index + step === files.length - 1
    const pressed = step < 0 ? previous.current : next.current
    if (landsOnEnd && document.activeElement === pressed)
      refocus.current = step < 0 ? 'next' : 'previous'
    onView(to)
  }
  useEffect(() => {
    const target = refocus.current
    if (!target) return
    refocus.current = null
    ;(target === 'next' ? next : previous).current?.focus()
  }, [index])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      // A focused clip seeks with the arrows; that is its own control and keeps them.
      if (event.defaultPrevented || event.target instanceof HTMLVideoElement) return
      const step = event.key === 'ArrowLeft' ? -1 : event.key === 'ArrowRight' ? 1 : 0
      const to = step ? files[index + step] : undefined
      if (!to) return
      event.preventDefault()
      onView(to)
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [files, index, onView, open])

  return (
    <Sheet
      open={open}
      labelledBy={headingId}
      size="wide"
      onClose={onClose}
      header={
        <div className="flex items-center gap-2">
          <Typography variant="label" as="h2" id={headingId} className="min-w-0 flex-1 truncate">
            {file}
          </Typography>
          {open && (
            <Typography variant="meta" as="span" className="shrink-0 tabular-nums">
              {t('attachmentViewer.position', {
                ns: 'posts',
                n: index + 1,
                total: files.length,
              })}
            </Typography>
          )}
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('action.close', { ns: 'common' })}
            onClick={onClose}
            className="shrink-0"
          >
            <X className="size-4" aria-hidden />
          </Button>
        </div>
      }
      footer={
        <div className="mt-3 flex gap-2 md:justify-end">
          <Button
            ref={previous}
            variant="secondary"
            className="flex-1 md:flex-none"
            disabled={!open || index <= 0}
            onClick={() => go(-1)}
          >
            <ChevronLeft className="size-4" aria-hidden />
            {t('attachmentViewer.previous', { ns: 'posts' })}
          </Button>
          <Button
            ref={next}
            variant="secondary"
            className="flex-1 md:flex-none"
            disabled={!open || index >= files.length - 1}
            onClick={() => go(1)}
          >
            {t('attachmentViewer.next', { ns: 'posts' })}
            <ChevronRight className="size-4" aria-hidden />
          </Button>
        </div>
      }
    >
      <div className="mt-3 flex justify-center">
        {attachment?.viewUrl &&
          (attachment.kind === 'video' ? (
            <video
              // A new clip is a new element, so the one before stops rather than keeps playing.
              key={attachment.viewUrl}
              src={attachment.viewUrl}
              controls
              playsInline
              preload="metadata"
              className="bg-surface-recessed max-h-media-view w-full rounded-lg object-contain"
            >
              {attachment.contentType && (
                <source src={attachment.viewUrl} type={attachment.contentType} />
              )}
            </video>
          ) : (
            <RotatedImage
              fit="natural"
              rotation={attachment.rotation}
              maxFrameHeight="var(--spacing-media-view)"
              frameClassName="mx-auto rounded-lg"
              src={attachment.viewUrl}
              alt={attachment.filename}
              width={attachment.width}
              height={attachment.height}
              decoding="async"
              className="max-h-media-view h-auto w-auto max-w-full rounded-lg object-contain"
            />
          ))}
      </div>
    </Sheet>
  )
}
