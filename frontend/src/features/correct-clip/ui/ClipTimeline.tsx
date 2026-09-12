import { useEffect, useRef } from 'react'
import { Film } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  timelineCuts,
  clipTextTracks,
  type ClipEditPlan,
  type ClipSelection,
} from '@/entities/clip-project'
import { Button, Typography } from '@/shared/ui'
import { CLIP_TIMELINE } from '@/shared/config'

export function ClipTimeline({
  plan,
  selection,
  timeMs,
  onSelect,
  localSources,
}: {
  plan: ClipEditPlan
  selection?: ClipSelection
  timeMs: number
  onSelect: (selection: ClipSelection) => void
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
}) {
  const { t } = useTranslation('clips')
  const strip = useRef<HTMLDivElement>(null)
  const cuts = timelineCuts(plan)
  const order = plan.cuts.map((cut) => cut.id).join(',')
  const phrase = selection?.kind === 'text' ? selection.phrase : undefined
  const duration = Number.isFinite(plan.durationMs) ? Math.max(1, plan.durationMs) : 1
  const width = Math.max(CLIP_TIMELINE.minWidth, (duration / 1000) * CLIP_TIMELINE.pixelsPerSecond)
  const left = (ms: number) => (Number.isFinite(ms) ? (ms / duration) * 100 : 0)
  // Explicit selection scrolls the horizontal timeline only. Playback never
  // moves the document or steals focus from a text field.
  useEffect(() => {
    const el = strip.current?.querySelector<HTMLElement>('[aria-pressed="true"]')
    const parent = strip.current
    if (el && parent) {
      const bounds = el.getBoundingClientRect(),
        viewport = parent.getBoundingClientRect()
      if (bounds.left < viewport.left || bounds.right > viewport.right)
        parent.scrollLeft += bounds.left - viewport.left
    }
  }, [selection?.id, selection?.kind, phrase, order])
  const tracks = clipTextTracks(plan)
  return (
    <div
      ref={strip}
      className="min-w-0 overflow-x-auto overscroll-x-contain py-2"
      aria-label={t('timeline.label')}
    >
      <div className="relative space-y-2" style={{ width }}>
        <div className="relative h-6" aria-hidden="true">
          {cuts.map(({ cut, startMs }) => (
            <Typography
              key={cut.id}
              variant="meta"
              className="absolute"
              style={{ left: `${left(startMs)}%` }}
            >
              {clipSeconds(startMs)} s
            </Typography>
          ))}
        </div>
        <ul className="relative h-24" aria-label={t('timeline.cuts')}>
          {cuts.map(({ cut, index, startMs, endMs }) => {
            const active = startMs <= timeMs && timeMs < endMs
            const url = localSources.find((s) => s.fingerprint === cut.fingerprint)?.url
            const selected = selection?.kind === 'cut' && selection.id === cut.id
            return (
              <li
                key={cut.id}
                className="absolute h-full px-1"
                style={{
                  left: `${left(startMs)}%`,
                  width: `${Math.max(0, left(endMs - startMs))}%`,
                }}
              >
                <Button
                  variant={selected ? 'secondary' : 'ghost'}
                  className="h-full w-full overflow-hidden"
                  aria-pressed={selected}
                  aria-current={active ? 'time' : undefined}
                  onClick={() => onSelect({ kind: 'cut', id: cut.id })}
                >
                  <span className="flex min-w-0 flex-col items-center gap-1">
                    {url && (active || selected) ? (
                      <video
                        muted
                        playsInline
                        preload="metadata"
                        aria-hidden="true"
                        tabIndex={-1}
                        src={`${url}#t=${cut.startMs / 1000}`}
                        className="h-10 w-16 rounded-sm object-cover"
                      />
                    ) : (
                      <Film className="size-6" aria-hidden="true" />
                    )}
                    <Typography as="span" variant="meta" className="truncate">
                      {t('correction.cut', { number: index + 1 })}
                    </Typography>
                  </span>
                </Button>
              </li>
            )
          })}
        </ul>
        {tracks.map((track, lane) => (
          <ul key={lane} className="relative h-11">
            {track.map((bar) => (
              <li
                key={`${bar.id}-${bar.phrase ?? 'text'}`}
                className="absolute h-11"
                style={{
                  left: `${left(bar.startMs)}%`,
                  width: `${Math.max(0, left(bar.endMs - bar.startMs))}%`,
                }}
              >
                <Button
                  variant={
                    selection?.kind === 'text' &&
                    selection.id === bar.id &&
                    selection.phrase === bar.phrase
                      ? 'secondary'
                      : 'ghost'
                  }
                  className="h-11 w-full overflow-hidden"
                  aria-pressed={
                    selection?.kind === 'text' &&
                    selection.id === bar.id &&
                    selection.phrase === bar.phrase
                  }
                  onClick={() => onSelect({ kind: 'text', id: bar.id, phrase: bar.phrase })}
                >
                  <Typography as="span" variant="meta" className="truncate">
                    {bar.text}
                  </Typography>
                </Button>
              </li>
            ))}
          </ul>
        ))}
        <div
          aria-hidden="true"
          className="bg-content-primary pointer-events-none absolute inset-y-0 w-px"
          style={{ left: `${left(timeMs)}%` }}
        />
      </div>
    </div>
  )
}
