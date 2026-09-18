import { useRef, useState } from 'react'
import { Film, GripVertical } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatDuration } from '@/shared/lib/media'
import { Button, FieldLabel, Listbox, Switch, Typography } from '@/shared/ui'
import { moveInOrder, reorderTargetIndex } from '../model/source-order'

interface SourceTile {
  fingerprint: string
  filename: string
  durationMs: number
  previewURL?: string
  status?: string
  retainOriginalAudio?: boolean
  soundDisabled?: boolean
  /** The item this whole source is bound to, empty when the owner left it for
   *  automatic association (CLIP-123). */
  boundItem?: string
}

/** The same source selector serves current uploads and retained observations.
 * Thumbnails never play; the selected source's player belongs below the strip. */
export function ClipSourceStrip({
  sources,
  selected,
  onSelect,
  label,
  onSoundChange,
  items,
  onItemChange,
  onReorder,
}: {
  sources: readonly SourceTile[]
  selected: string
  onSelect: (fingerprint: string) => void
  label: string
  onSoundChange?: (fingerprint: string, enabled: boolean) => void
  /** The items of the project's declared groups. A project that declares none
   *  offers nothing, so the control is absent rather than empty (CLIP-123). */
  items?: readonly { value: string; label: string }[]
  onItemChange?: (fingerprint: string, item: string) => void
  /** Moves one source to another place in the strip (CLIP-136). The whole new
   *  order is handed over, because that is what the server stores. Absent where
   *  the footage is only being looked at. */
  onReorder?: (fingerprints: string[]) => void
}) {
  const { t } = useTranslation('clips')
  const strip = useRef<HTMLUListElement>(null)
  const dragging = useRef<number | null>(null)
  const [announced, setAnnounced] = useState('')
  const order = sources.map((source) => source.fingerprint)
  const move = (from: number, to: number) => {
    if (!onReorder || to === from || to < 0 || to >= order.length) return
    onReorder(moveInOrder(order, from, to))
    setAnnounced(
      t('source.moved', {
        filename: sources[from].filename,
        position: to + 1,
        total: order.length,
      }),
    )
  }
  // The drag path and the two buttons mean the same thing, which is why they
  // share `move`: a pointer drop names a target index, a button names the one
  // beside it (CLIP-55).
  const drop = (from: number, clientX: number) => {
    const boxes = Array.from(strip.current?.children ?? []).map((tile) => {
      const box = tile.getBoundingClientRect()
      return { left: box.left, right: box.right }
    })
    move(from, reorderTargetIndex(boxes, clientX))
  }
  return (
    <div className="min-w-0 space-y-2">
      <ul
        ref={strip}
        aria-label={label}
        className="-mx-4 flex snap-x snap-mandatory scroll-px-4 gap-2 overflow-x-auto overscroll-x-contain px-4 py-2 sm:mx-0 sm:scroll-px-2 sm:px-2"
      >
        {sources.map((source, index) => (
          <li
            key={source.fingerprint}
            className="w-40 shrink-0 snap-start"
            onPointerUp={(event) => {
              if (dragging.current !== null) drop(dragging.current, event.clientX)
              dragging.current = null
            }}
          >
            <Button
              variant={source.fingerprint === selected ? 'secondary' : 'ghost'}
              className="w-full"
              aria-pressed={source.fingerprint === selected}
              aria-label={t('source.inspect', { filename: source.filename })}
              onClick={() => onSelect(source.fingerprint)}
            >
              <span className="flex w-32 min-w-0 flex-col gap-2 py-3 text-left">
                <span className="bg-surface-recessed relative flex aspect-video items-center justify-center overflow-hidden rounded-md">
                  {source.previewURL ? (
                    <video
                      key={source.previewURL}
                      src={source.previewURL}
                      preload="metadata"
                      muted
                      playsInline
                      tabIndex={-1}
                      aria-hidden="true"
                      className="pointer-events-none absolute inset-0 h-full w-full object-cover"
                    />
                  ) : (
                    <Film aria-hidden="true" className="text-content-tertiary size-6" />
                  )}
                </span>
                <Typography variant="label" className="line-clamp-2 break-all">
                  {source.filename}
                </Typography>
                <Typography variant="meta" className="text-content-secondary">
                  {formatDuration(source.durationMs)}
                </Typography>
                {source.status && (
                  <Typography variant="meta" className="break-words">
                    {source.status}
                  </Typography>
                )}
              </span>
            </Button>
            {onReorder && order.length > 1 && (
              <div className="flex items-center gap-1 px-2">
                {/* The handle starts a drag; the two buttons do the same move
                    without one, for touch and for the keyboard (CLIP-55). */}
                <span
                  aria-hidden="true"
                  className="cursor-grab touch-none px-1 py-2"
                  onPointerDown={() => {
                    dragging.current = index
                  }}
                >
                  <GripVertical className="text-content-tertiary size-4" />
                </span>
                <Button
                  variant="ghost"
                  disabled={index === 0}
                  aria-label={t('source.moveEarlierName', { filename: source.filename })}
                  onClick={() => move(index, index - 1)}
                >
                  {t('source.moveEarlier')}
                </Button>
                <Button
                  variant="ghost"
                  disabled={index === order.length - 1}
                  aria-label={t('source.moveLaterName', { filename: source.filename })}
                  onClick={() => move(index, index + 1)}
                >
                  {t('source.moveLater')}
                </Button>
              </div>
            )}
            {onItemChange && !!items?.length && (
              <div className="min-w-0 px-2 py-2">
                <FieldLabel htmlFor={`clip-source-item-${source.fingerprint}`}>
                  {t('source.boundItem')}
                </FieldLabel>
                <Typography variant="meta" className="text-content-secondary mt-1 block">
                  {t('source.boundItemHelp')}
                </Typography>
                <Listbox
                  id={`clip-source-item-${source.fingerprint}`}
                  className="mt-2"
                  aria-label={t('source.boundItemName', { filename: source.filename })}
                  value={source.boundItem ?? ''}
                  onChange={(value) => onItemChange(source.fingerprint, value)}
                  options={[{ value: '', label: t('source.boundItemNone') }, ...items]}
                />
              </div>
            )}
            {onSoundChange && (
              <label className="flex min-h-11 cursor-pointer items-center gap-2 px-2 py-3">
                <Switch
                  checked={source.retainOriginalAudio ?? false}
                  aria-checked={source.retainOriginalAudio ?? false}
                  aria-label={t('source.originalSoundName', { filename: source.filename })}
                  disabled={source.soundDisabled}
                  onChange={(event) => onSoundChange(source.fingerprint, event.target.checked)}
                />
                <Typography variant="meta" className="min-w-0 break-words">
                  {t('source.originalSound')}
                </Typography>
              </label>
            )}
          </li>
        ))}
      </ul>
      <Typography variant="meta" role="status" aria-live="polite" className="block">
        {announced}
      </Typography>
      <Typography variant="meta" className="text-content-secondary block">
        {t('source.position', {
          current: sources.findIndex((source) => source.fingerprint === selected) + 1,
          total: sources.length,
        })}
      </Typography>
    </div>
  )
}
