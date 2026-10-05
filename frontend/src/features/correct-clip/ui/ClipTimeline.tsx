import { useEffect, useRef } from 'react'
import { Film, Redo2, Undo2, GripVertical, ArrowLeft, ArrowRight, Scissors } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  clipTextTracks,
  cutRate,
  narrationSlot,
  timelineCuts,
  timelineLabelFits,
  spokenState,
  timelineBarPx,
  type TimelineEdit,
  type ClipEditPlan,
  type ClipSelection,
} from '@/entities/clip-plan'
import { type ClipNotice } from '@/entities/clip-project'
import { Button, Typography } from '@/shared/ui'
import { useTimelineGesture } from '../model/useTimelineGesture'
import { cutGestureEdit, type SourceBounds, type CutGesture } from '../model/timeline-gesture'
import { CLIP_TIMELINE } from '@/entities/clip-design'
export function ClipTimeline({
  plan,
  selection,
  timeMs,
  onSelect,
  onAddCaption,
  onAddDubbing,
  readOnly = false,
  disabled = false,
  localSources,
  onSeek,
  onCommit,
  onPreview,
  bounds,
  onSplit,
  canSplit = true,
  history,
  notices = [],
}: {
  notices?: readonly ClipNotice[]
  plan: ClipEditPlan
  selection?: ClipSelection
  timeMs: number
  onSelect: (selection: ClipSelection) => void
  /** Adds an independent caption at the playhead. */
  onAddCaption?: (slot: { startMs: number; endMs: number }) => void
  onAddDubbing?: () => void
  readOnly?: boolean
  disabled?: boolean
  onSeek?: (ms: number) => void
  onCommit?: (edit: TimelineEdit) => void
  onPreview?: (plan?: ClipEditPlan) => void
  bounds?: (id: string) => SourceBounds
  canSplit?: boolean
  onSplit?: () => void
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
  const canvas = useRef<HTMLDivElement>(null)
  const { begin, move, end, cancel, seekAt } = useTimelineGesture({
    canvas,
    plan,
    timeMs,
    onSeek,
    onCommit: readOnly || disabled ? undefined : onCommit,
    onPreview,
    bounds,
  })
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
  const slot = onAddCaption && plan.nativeComposition ? narrationSlot(plan, timeMs) : undefined
  // Every label is bounded by the bar it belongs to, so the only question left
  // is whether the bar is wide enough to hold one at all (CLIP-54).
  const fits = (spanMs: number) => timelineLabelFits(spanMs, duration, width)
  const handles = (id: string, index: number) =>
    (['start', 'move', 'end'] as const).map((intent: CutGesture) => {
      const Icon = intent === 'start' ? ArrowLeft : intent === 'end' ? ArrowRight : GripVertical
      return (
        <Button
          key={intent}
          variant="secondary"
          size="icon"
          className="touch-none"
          disabled={disabled}
          aria-label={t(`timeline.handles.${intent}`, { number: index + 1 })}
          onPointerDown={(event) => begin(event, id, intent)}
          onPointerMove={move}
          onPointerUp={end}
          onPointerCancel={cancel}
          onKeyDown={(event) => {
            if (
              !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key) ||
              !onCommit
            )
              return
            event.preventDefault()
            const direction = event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 1
            const edit =
              intent === 'move'
                ? {
                    type: 'move' as const,
                    from: index,
                    to: Math.max(0, Math.min(cuts.length - 1, index + direction)),
                  }
                : cutGestureEdit(
                    plan,
                    id,
                    intent,
                    direction * (event.shiftKey ? 1000 : 1000 / CLIP_TIMELINE.framesPerSecond),
                    0,
                    bounds?.(id) ?? { startMs: 0, endMs: 0 },
                  )
            if (edit) onCommit(edit)
          }}
        >
          <Icon className="size-5" aria-hidden="true" />
        </Button>
      )
    })
  const selectedCut = cuts.find((c) => selection?.kind === 'cut' && c.cut.id === selection.id)
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
      {!readOnly && selectedCut && onCommit && (
        <div className="flex flex-wrap items-center gap-2">
          {timelineBarPx(selectedCut.endMs - selectedCut.startMs, duration, width) < 148 &&
            handles(selectedCut.cut.id, selectedCut.index)}
          {onSplit && (
            <Button variant="secondary" onClick={onSplit} disabled={disabled || !canSplit}>
              <Scissors className="size-5" aria-hidden="true" />
              {t('assembly.splitPlayhead')}
            </Button>
          )}
          <Typography variant="meta">{t('timeline.gestureHelp')}</Typography>
          {onSplit && !canSplit && (
            <Typography variant="meta">{t('assembly.splitInvalid')}</Typography>
          )}
        </div>
      )}
      <div
        ref={strip}
        className="min-w-0 overflow-x-auto overscroll-x-contain py-2"
        aria-label={t('timeline.label')}
      >
        <div
          ref={canvas}
          className="relative space-y-2"
          style={{ width }}
          onClick={(event) => {
            if (!(event.target as HTMLElement).closest('button,input,a')) seekAt(event.clientX)
          }}
        >
          {onSeek && (
            <Button
              size="icon"
              variant="secondary"
              className="absolute top-0 touch-none"
              style={{ left: Math.min(width - 44, Math.max(0, (width * left(timeMs)) / 100 - 22)) }}
              aria-label={t('timeline.playheadHandle')}
              onPointerDown={(event) => begin(event)}
              onPointerMove={move}
              onPointerUp={end}
              onPointerCancel={cancel}
            >
              <ArrowRight className="size-5" aria-hidden="true" />
            </Button>
          )}
          <div className="relative h-11" aria-hidden="true">
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
          <Typography variant="label" as="p">
            {t('timeline.trackVideo')}
          </Typography>
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
                  {!readOnly &&
                    selected &&
                    onCommit &&
                    timelineBarPx(endMs - startMs, duration, width) >= 148 && (
                      <div className="absolute inset-x-0 bottom-0 flex justify-between gap-2">
                        {handles(cut.id, index)}
                      </div>
                    )}
                </li>
              )
            })}
          </ul>
          <Typography variant="label" as="p">
            {t('timeline.trackCaptions')}
          </Typography>
          {!readOnly && (
            <Button
              variant="secondary"
              disabled={disabled || !slot}
              onClick={() => slot && onAddCaption?.(slot)}
            >
              {t('timeline.addCaption')}
            </Button>
          )}
          {!readOnly && !slot && (
            <Typography variant="meta">{t('timeline.captionUnavailable')}</Typography>
          )}
          {tracks.length === 0 && (
            <ul className="relative h-11" aria-label={t('timeline.captionTrack')}>
              <li>
                <Typography variant="meta">{t('timeline.emptyCaptions')}</Typography>
              </li>
            </ul>
          )}
          {tracks.map((track, lane) => (
            <ul
              key={lane}
              className="relative h-11"
              aria-label={lane === 0 ? t('timeline.captionTrack') : undefined}
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
                    aria-invalid={bar.invalid || undefined}
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
                        {bar.invalid && ` · ${t('assembly.invalidRange')}`}
                      </Typography>
                    )}
                  </Button>
                </li>
              ))}
            </ul>
          ))}
          <Typography variant="label" as="p">
            {t('timeline.trackDubbing')}
          </Typography>
          {!readOnly && (
            <Button variant="secondary" disabled={disabled || !onAddDubbing} onClick={onAddDubbing}>
              {t('timeline.addDubbing')}
            </Button>
          )}
          {!readOnly && !onAddDubbing && (
            <Typography variant="meta">{t('timeline.dubbingUnavailable')}</Typography>
          )}
          <ul className="relative h-11" aria-label={t('timeline.dubbingTrack')}>
            {(plan.narration?.segments ?? []).map((segment, index) => {
              const state = spokenState(plan.narration!, segment, plan.durationMs)
              const selected = selection?.kind === 'spoken' && selection.id === segment.id
              return (
                <li
                  key={segment.id}
                  className="absolute h-11"
                  style={{
                    left: `${left(segment.startMs)}%`,
                    width: `${Math.max(0, left(segment.endMs - segment.startMs))}%`,
                  }}
                >
                  <Button
                    variant={selected ? 'secondary' : 'ghost'}
                    className="h-11 w-full overflow-hidden"
                    aria-pressed={selected}
                    aria-label={`${t('timeline.spokenSegment', { number: index + 1 })} · ${t(`timeline.speechState.${state}`)}`}
                    onClick={() => onSelect({ kind: 'spoken', id: segment.id })}
                  >
                    {fits(segment.endMs - segment.startMs) && (
                      <Typography as="span" variant="meta" className="w-full truncate">
                        {segment.text} · {t(`timeline.speechState.${state}`)}
                      </Typography>
                    )}
                  </Button>
                </li>
              )
            })}
            {!plan.narration?.segments.length && (
              <li>
                <Typography variant="meta">{t('timeline.emptyDubbing')}</Typography>
              </li>
            )}
          </ul>
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
