import { useEffect, useRef } from 'react'
import { Film, Redo2, Undo2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  type ClipNotice,
  clipSeconds,
  timelineCuts,
  timelineLabelFits,
  clipTextTracks,
  narrationSlot,
  cutRate,
  type ClipEditPlan,
  type ClipSelection,
} from '@/entities/clip-project'
import { Button, Typography } from '@/shared/ui'
import { CLIP_TIMELINE } from '@/entities/clip-project'
export function ClipTimeline({
  plan,
  selection,
  timeMs,
  onSelect,
  onAddCaption,
  localSources,
  history,
  notices = [],
}: {
  notices?: readonly ClipNotice[]
  plan: ClipEditPlan
  selection?: ClipSelection
  timeMs: number
  onSelect: (selection: ClipSelection) => void
  /** Adds a caption to the narration at the playhead. Absent where the plan
   *  carries no narration to edit. */
  onAddCaption?: (slot: { startMs: number; endMs: number }) => void
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  /** Undo/redo for every edit the timeline commits (CLIP-55). They head the
   *  timeline because that is the track they act on, and they are icons because
   *  the row they sit in is the one the cut controls also reach for. */
  history?: {
    undo: () => void
    redo: () => void
    canUndo: boolean
    canRedo: boolean
    disabled?: boolean
  }
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
  const captions = (plan.elements ?? []).some((text) => text.narration)
  const slot = onAddCaption && captions ? narrationSlot(plan, timeMs) : undefined
  // Every label is bounded by the bar it belongs to, so the only question left
  // is whether the bar is wide enough to hold one at all (CLIP-54).
  const fits = (spanMs: number) => timelineLabelFits(spanMs, duration, width)
  return (
    <div className="min-w-0 space-y-2">
      {history && (
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('timeline.undo')}
            disabled={history.disabled || !history.canUndo}
            onClick={history.undo}
          >
            <Undo2 className="size-5" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('timeline.redo')}
            disabled={history.disabled || !history.canRedo}
            onClick={history.redo}
          >
            <Redo2 className="size-5" aria-hidden="true" />
          </Button>
        </div>
      )}
      <div
        ref={strip}
        className="min-w-0 overflow-x-auto overscroll-x-contain py-2"
        aria-label={t('timeline.label')}
      >
        <div className="relative space-y-2" style={{ width }}>
          <div className="relative h-6" aria-hidden="true">
            {cuts.map(({ cut, startMs, endMs }) =>
              fits(endMs - startMs) ? (
                <Typography
                  key={cut.id}
                  variant="meta"
                  as="span"
                  className="absolute block truncate"
                  style={{
                    left: `${left(startMs)}%`,
                    width: `${Math.max(0, left(endMs - startMs))}%`,
                  }}
                >
                  {clipSeconds(startMs)} s
                </Typography>
              ) : null,
            )}
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
                    // The NAME is the control's, not the drawn label's: a cut too
                    // narrow to print its name is still one a reader reaches by it.
                    aria-label={t('correction.cut', { number: index + 1 })}
                    aria-pressed={selected}
                    aria-current={active ? 'time' : undefined}
                    onClick={() => onSelect({ kind: 'cut', id: cut.id })}
                  >
                    <span className="flex min-w-0 flex-col items-center gap-1">
                      {/* Drawn from the moment the track is (CLIP-54): waiting for the
                        playhead or a click left a strip of identical glyphs, which is
                        the one thing a cut list has to tell apart. The glyph stays
                        where this fingerprint has no playable source at all. */}
                      {url ? (
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
                      {fits(endMs - startMs) && (
                        <>
                          <Typography as="span" variant="meta" className="w-full truncate">
                            {t('correction.cut', { number: index + 1 })}
                          </Typography>
                          <Typography as="span" variant="meta" className="w-full truncate">
                            {cutRate(cut) / 1000}×
                            {notices.some((n) => n.cutId === cut.id) && ` · ${t('notices.marker')}`}
                          </Typography>
                        </>
                      )}
                    </span>
                  </Button>
                </li>
              )
            })}
          </ul>
          {onAddCaption && captions && (
            <Button variant="secondary" disabled={!slot} onClick={() => slot && onAddCaption(slot)}>
              {t('timeline.addCaption')}
            </Button>
          )}
          {tracks.map((track, lane) => (
            <ul
              key={lane}
              className="relative h-11"
              aria-label={lane === 0 && captions ? t('timeline.captionTrack') : undefined}
            >
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
                    aria-label={bar.text}
                    aria-pressed={
                      selection?.kind === 'text' &&
                      selection.id === bar.id &&
                      selection.phrase === bar.phrase
                    }
                    onClick={() => onSelect({ kind: 'text', id: bar.id, phrase: bar.phrase })}
                  >
                    {fits(bar.endMs - bar.startMs) && (
                      <Typography as="span" variant="meta" className="w-full truncate">
                        {bar.text}
                      </Typography>
                    )}
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
    </div>
  )
}
