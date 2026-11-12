import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronDown, ChevronUp, GripVertical, Redo2, SlidersHorizontal, Undo2 } from 'lucide-react'
import {
  clipSeconds,
  clipSourceSound,
  cutRate,
  spokenState,
  textInterval,
  type ClipEditPlan,
  type ClipEditableText,
  type ClipSelection,
  type RetainedClipSource,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { type ClipNotice } from '@/entities/clip-project'
import { Badge, Button, FieldMessage, Textarea, Typography, cardStyles } from '@/shared/ui'
import { clipSceneBlocks } from '../model/scene-blocks'
import { ClipCutSourceFrame } from './ClipCutSourceFrame'

type Validation = { elements: ReadonlyArray<{ id: string; interval: boolean; text: boolean }> }

/** ②'s plan as one vertical list of scene blocks (CLIP-17, THEME-44): the intro first, one block
 *  per cut in output order with the captions and spoken segments that start inside it — their
 *  text edited in place — then the outro and the disclosure badge. Selecting a block seeks the
 *  preview and marks it; playback marks the block it plays and never moves focus (CLIP-53). Each
 *  block, caption and spoken segment opens its own sheet from its own control (CLIP-55), and the
 *  cut blocks reorder by a drag handle, move buttons and Alt+ArrowUp/Down, each move announced
 *  and undoable (THEME-32/33). */
export function ClipSceneBlocks({
  plan,
  sources,
  selection,
  timeMs,
  onSelect,
  onOpen,
  onMove,
  change,
  validation,
  footage,
  intro,
  outro,
  history,
  notices = [],
  readOnly = false,
  disabled = false,
}: {
  plan: ClipEditPlan
  sources: readonly Pick<RetainedClipSource, 'id' | 'filename'>[]
  selection?: ClipSelection
  timeMs: number
  onSelect: (selection: ClipSelection) => void
  /** Selects the item and opens its own sheet. */
  onOpen?: (selection: ClipSelection) => void
  /** Moves a cut `delta` places; false when it cannot move. */
  onMove?: (id: string, delta: number) => boolean
  change: (edit: TimelineEdit, group?: string) => void
  validation?: Validation
  footage: {
    localSources: ReadonlyArray<{ fingerprint: string; url: string }>
    resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  }
  /** The project's intro and outro slots, edited where the storyline edits them (CLIP-188). */
  intro?: ReactNode
  outro?: ReactNode
  history?: {
    undo: () => void
    redo: () => void
    canUndo: boolean
    canRedo: boolean
    disabled?: boolean
  }
  notices?: readonly ClipNotice[]
  readOnly?: boolean
  disabled?: boolean
}) {
  const { t } = useTranslation('clips')
  const blocks = clipSceneBlocks(plan)
  const list = useRef<HTMLOListElement>(null)
  const playing = blocks.findLastIndex((b) => b.cut.startMs <= timeMs)
  const order = plan.cuts.map((cut) => cut.id).join(',')
  const [announcement, setAnnouncement] = useState('')
  const [dragging, setDragging] = useState<string>()
  // A moved block is re-rendered in its new place; the control that moved it keeps focus there.
  const refocus = useRef<{ id: string; control: string }>(undefined)
  useEffect(() => {
    const target = refocus.current
    if (!target) return
    refocus.current = undefined
    // A move button at the new end is disabled; its opposite, then the block itself, takes focus.
    const fallback = { up: 'down', down: 'up' }[target.control] ?? 'select'
    for (const control of [target.control, fallback, 'select']) {
      const element = list.current?.querySelector<HTMLElement>(
        `[data-cut-id="${target.id}"] [data-block-control="${control}"]:not(:disabled)`,
      )
      if (element) return element.focus()
    }
  }, [order])
  const move = (id: string, delta: number, control: string) => {
    const from = plan.cuts.findIndex((c) => c.id === id)
    if (!onMove || from < 0 || !onMove(id, delta)) return
    refocus.current = { id, control }
    setAnnouncement(
      t('sceneBlocks.moved', {
        number: from + 1,
        position: from + delta + 1,
        count: blocks.length,
      }),
    )
  }
  const keyMove = (event: KeyboardEvent<HTMLElement>, id: string) => {
    if (!event.altKey || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return
    const target = event.target as HTMLElement
    // A text field keeps its own Alt+Arrow caret movement.
    if (target instanceof HTMLTextAreaElement || target instanceof HTMLInputElement) return
    event.preventDefault()
    move(
      id,
      event.key === 'ArrowUp' ? -1 : 1,
      target.closest('[data-block-control]')?.getAttribute('data-block-control') ?? 'select',
    )
  }
  const reorderable = !!onMove && !readOnly && !disabled
  // An explicit selection brings its block into view; playback marks without scrolling.
  useEffect(() => {
    const block = list.current?.querySelector<HTMLElement>('[data-selected="true"]')
    if (block && !block.contains(document.activeElement))
      block.scrollIntoView?.({ block: 'nearest', behavior: 'instant' })
  }, [selection?.id, selection?.kind, order])
  const badge = (plan.elements ?? []).find((e) => e.role === 'badge' || e.role === 'disclosure')
  const badgeInterval = badge ? textInterval(plan, badge) : undefined
  const fieldError = (id: string) => validation?.elements.find((e) => e.id === id)
  const locked = readOnly || disabled
  const textSelected = (id: string) => selection?.kind === 'text' && selection.id === id
  /** A caption's or the badge's words, edited where they stand (CLIP-17); its validation message
   *  stands under the field that failed. */
  const inPlaceText = (text: ClipEditableText, label: string) => {
    const error = fieldError(text.instanceId)
    const invalid = !!(error?.text || error?.interval)
    const id = `scene-text-${text.instanceId}`
    return (
      <div key={text.instanceId} className="min-w-0 space-y-1" data-scene-text={text.instanceId}>
        <div className="flex min-w-0 items-center justify-between gap-2">
          <Typography variant="meta" as="label" htmlFor={id} className="min-w-0">
            {label}
          </Typography>
          {onOpen && (
            <Button
              variant="ghost"
              size="icon"
              aria-label={t('sceneBlocks.openText', { label })}
              onClick={() => onOpen({ kind: 'text', id: text.instanceId })}
            >
              <SlidersHorizontal aria-hidden="true" className="size-5" />
            </Button>
          )}
        </div>
        <Textarea
          id={id}
          autoGrow
          rows={1}
          value={text.text}
          readOnly={locked}
          aria-invalid={invalid || undefined}
          aria-describedby={invalid ? `${id}-error` : undefined}
          data-selected={textSelected(text.instanceId)}
          onFocus={() => {
            if (!textSelected(text.instanceId)) onSelect({ kind: 'text', id: text.instanceId })
          }}
          onChange={(event) =>
            change(
              { type: 'text', id: text.instanceId, patch: { text: event.target.value } },
              `${text.instanceId}:text`,
            )
          }
        />
        {invalid && (
          <FieldMessage id={`${id}-error`}>
            {t(error?.text ? 'sceneBlocks.textTooLong' : 'timeline.captionAttention')}
          </FieldMessage>
        )}
      </div>
    )
  }
  return (
    <section className="min-w-0 space-y-3" aria-label={t('sceneBlocks.label')}>
      {/* Mounted before any move so the first announcement is read (THEME-33). */}
      <p role="status" className="sr-only">
        {announcement}
      </p>
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
      <ol ref={list} className="min-w-0 space-y-3">
        {intro && (
          <li className={cardStyles({ className: 'min-w-0 space-y-2' })} data-scene-block="intro">
            <Typography variant="fieldTitle" as="h3">
              {t('sceneBlocks.intro')}
            </Typography>
            {intro}
          </li>
        )}
        {blocks.map(({ cut: item, captions, spoken }) => {
          const { cut, index, startMs, endMs } = item
          const selected =
            (selection?.kind === 'cut' && selection.id === cut.id) ||
            (selection?.kind === 'text' && captions.some((c) => c.instanceId === selection.id)) ||
            (selection?.kind === 'spoken' && spoken.some((s) => s.id === selection.id))
          const source = sources.find((s) => s.id === cut.sourceId)
          const rate = cutRate(cut) / 1000
          const sound = clipSourceSound(plan, cut)
          return (
            <li
              key={cut.id}
              className={cardStyles({
                className: `min-w-0 space-y-3 ${selected ? 'ring-stroke-accent ring-2' : ''}`,
              })}
              data-scene-block="cut"
              data-cut-id={cut.id}
              data-selected={selected}
              data-dragging={dragging === cut.id || undefined}
              aria-current={index === playing ? 'time' : undefined}
              onKeyDown={reorderable ? (event) => keyMove(event, cut.id) : undefined}
              onDragOver={(event) => {
                if (dragging && dragging !== cut.id) event.preventDefault()
              }}
              onDrop={(event) => {
                if (!dragging) return
                event.preventDefault()
                const from = plan.cuts.findIndex((c) => c.id === dragging)
                setDragging(undefined)
                if (from >= 0 && from !== index) move(dragging, index - from, 'handle')
              }}
            >
              <div className="flex min-w-0 items-start gap-3">
                <span className="bg-surface-recessed flex aspect-video w-24 shrink-0 items-center justify-center overflow-hidden rounded-sm">
                  <ClipCutSourceFrame
                    still
                    cut={cut}
                    localSources={footage.localSources}
                    resolvePlayback={footage.resolvePlayback}
                  />
                </span>
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-1">
                    <Button
                      variant={selected ? 'secondary' : 'ghost'}
                      data-block-control="select"
                      aria-pressed={selection?.kind === 'cut' && selection.id === cut.id}
                      aria-keyshortcuts={reorderable ? 'Alt+ArrowUp Alt+ArrowDown' : undefined}
                      onClick={() => onSelect({ kind: 'cut', id: cut.id })}
                    >
                      {t('correction.cut', { number: index + 1 })}
                    </Button>
                    {onOpen && (
                      <Button
                        variant="ghost"
                        size="icon"
                        data-block-control="open"
                        aria-label={t('sceneBlocks.openCut', { number: index + 1 })}
                        onClick={() => onOpen({ kind: 'cut', id: cut.id })}
                      >
                        <SlidersHorizontal aria-hidden="true" className="size-5" />
                      </Button>
                    )}
                  </div>
                  <Typography variant="meta" as="p">
                    {t('sceneBlocks.output', {
                      start: clipSeconds(startMs),
                      end: clipSeconds(endMs),
                    })}
                  </Typography>
                  <Typography variant="meta" as="p" className="break-words">
                    {t('sceneBlocks.source', {
                      name: source?.filename ?? cut.sourceId,
                      start: clipSeconds(cut.startMs),
                      end: clipSeconds(cut.endMs),
                    })}
                  </Typography>
                  <div className="flex flex-wrap items-center gap-2">
                    {rate !== 1 && <Badge tone="accent">{t('sceneBlocks.rate', { rate })}</Badge>}
                    <Typography variant="meta" as="span">
                      {t(sound ? 'sceneBlocks.soundOn' : 'sceneBlocks.soundOff')}
                    </Typography>
                    {notices.some((n) => n.cutId === cut.id) && (
                      <Badge tone="warning">{t('notices.marker')}</Badge>
                    )}
                  </div>
                </div>
                {reorderable && (
                  <div className="flex shrink-0 flex-col items-center gap-1">
                    {/* Dragging is never the only way (THEME-32): the move buttons below do the
                        same on touch, and Alt+ArrowUp/Down on the focused block from a keyboard. */}
                    <Button
                      variant="ghost"
                      size="icon"
                      draggable
                      data-block-control="handle"
                      className="cursor-grab active:cursor-grabbing"
                      aria-label={t('sceneBlocks.drag', { number: index + 1 })}
                      aria-keyshortcuts="Alt+ArrowUp Alt+ArrowDown"
                      onDragStart={(event) => {
                        event.dataTransfer?.setData('text/plain', cut.id)
                        setDragging(cut.id)
                      }}
                      onDragEnd={() => setDragging(undefined)}
                    >
                      <GripVertical aria-hidden="true" className="size-5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      data-block-control="up"
                      disabled={index === 0}
                      aria-label={t('sceneBlocks.up', { number: index + 1 })}
                      onClick={() => move(cut.id, -1, 'up')}
                    >
                      <ChevronUp aria-hidden="true" className="size-5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      data-block-control="down"
                      disabled={index === blocks.length - 1}
                      aria-label={t('sceneBlocks.down', { number: index + 1 })}
                      onClick={() => move(cut.id, 1, 'down')}
                    >
                      <ChevronDown aria-hidden="true" className="size-5" />
                    </Button>
                  </div>
                )}
              </div>
              {captions.map((text) => {
                const interval = textInterval(plan, text)
                return inPlaceText(
                  text,
                  t('sceneBlocks.caption', {
                    start: clipSeconds(interval.startMs),
                    end: clipSeconds(interval.endMs),
                  }),
                )
              })}
              {spoken.map((segment) => {
                const number = (plan.narration?.segments.indexOf(segment) ?? 0) + 1
                const id = `scene-spoken-${segment.id}`
                const state = spokenState(plan.narration!, segment, plan.durationMs)
                return (
                  <div key={segment.id} className="min-w-0 space-y-1">
                    <div className="flex min-w-0 items-center justify-between gap-2">
                      <Typography variant="meta" as="label" htmlFor={id} className="min-w-0">
                        {`${t('timeline.spokenSegment', { number })} · ${t(`timeline.speechState.${state}`)}`}
                      </Typography>
                      {onOpen && (
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={t('sceneBlocks.openSpoken', { number })}
                          onClick={() => onOpen({ kind: 'spoken', id: segment.id })}
                        >
                          <SlidersHorizontal aria-hidden="true" className="size-5" />
                        </Button>
                      )}
                    </div>
                    <Textarea
                      id={id}
                      autoGrow
                      rows={1}
                      value={segment.text}
                      readOnly={locked}
                      data-selected={selection?.kind === 'spoken' && selection.id === segment.id}
                      onFocus={() => {
                        if (selection?.kind !== 'spoken' || selection.id !== segment.id)
                          onSelect({ kind: 'spoken', id: segment.id })
                      }}
                      onChange={(event) =>
                        change(
                          {
                            type: 'spokenPatch',
                            id: segment.id,
                            patch: { text: event.target.value },
                          },
                          `${segment.id}:text`,
                        )
                      }
                    />
                  </div>
                )
              })}
            </li>
          )
        })}
        {outro && (
          <li className={cardStyles({ className: 'min-w-0 space-y-2' })} data-scene-block="outro">
            <Typography variant="fieldTitle" as="h3">
              {t('sceneBlocks.outro')}
            </Typography>
            {outro}
          </li>
        )}
        {badge && badgeInterval && (
          <li
            className={cardStyles({
              className: `min-w-0 space-y-2 ${textSelected(badge.instanceId) ? 'ring-stroke-accent ring-2' : ''}`,
            })}
            data-scene-block="badge"
            data-selected={textSelected(badge.instanceId)}
          >
            <Typography variant="fieldTitle" as="h3">
              {t('sceneBlocks.badge')}
            </Typography>
            {inPlaceText(
              badge,
              t('sceneBlocks.output', {
                start: clipSeconds(badgeInterval.startMs),
                end: clipSeconds(badgeInterval.endMs),
              }),
            )}
          </li>
        )}
      </ol>
    </section>
  )
}
