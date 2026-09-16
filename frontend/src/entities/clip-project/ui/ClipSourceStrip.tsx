import { Film } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatDuration } from '@/shared/lib/video'
import { Button, FieldLabel, Listbox, Switch, Typography } from '@/shared/ui'

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
}) {
  const { t } = useTranslation('clips')
  return (
    <div className="min-w-0 space-y-2">
      <ul
        aria-label={label}
        className="-mx-4 flex snap-x snap-mandatory scroll-px-4 gap-2 overflow-x-auto overscroll-x-contain px-4 py-2 sm:mx-0 sm:scroll-px-2 sm:px-2"
      >
        {sources.map((source) => (
          <li key={source.fingerprint} className="w-40 shrink-0 snap-start">
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
            {onItemChange && !!items?.length && (
              <div className="min-w-0 px-2 py-2">
                <FieldLabel htmlFor={`clip-source-item-${source.fingerprint}`}>
                  {t('source.boundItem')}
                </FieldLabel>
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
      <Typography variant="meta" className="text-content-secondary block">
        {t('source.position', {
          current: sources.findIndex((source) => source.fingerprint === selected) + 1,
          total: sources.length,
        })}
      </Typography>
    </div>
  )
}
