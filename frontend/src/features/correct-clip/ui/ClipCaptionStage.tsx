import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ClipNoticeList,
  type ClipNotice,
  type ClipCanvasBox,
  type ClipCaptionFragment,
  type ClipEditableText,
  type TimelineEdit,
} from '@/entities/clip-project'
import { CLIP_CAPTION_PLACEMENT } from '@/entities/clip-project'
import { Typography } from '@/shared/ui'

/** Where the caption may sit: its measured bounds, moved — never resized — until
 *  they lie inside the safe area, which is the same rule the server applies when
 *  it stores the placement and the renderer applies when it lays it out
 *  (CDS-82). */
function clampToSafeArea(x: number, y: number, box: ClipCanvasBox, safe: ClipCanvasBox) {
  return {
    x: Math.round(Math.min(Math.max(x, safe.x), safe.x + safe.width - box.width)),
    y: Math.round(Math.min(Math.max(y, safe.y), safe.y + safe.height - box.height)),
  }
}

/**
 * ② places a caption over a still frame of the cut its interval starts in
 * (CLIP-143). The frame is the owner's own footage, played locally; the caption
 * is the SERVER's fragment, placed by one transform, so what is dragged here is
 * what the render draws (CDS-83).
 */
export function ClipCaptionStage({
  text,
  fragment,
  canvas,
  safeArea,
  frameUrl,
  frameFingerprint,
  frameStartMs,
  resolvePlayback,
  notices = [],
  language,
  change,
  disabled,
}: {
  text: ClipEditableText
  /** Absent while the server is still drawing it, or when it could not. */
  fragment?: ClipCaptionFragment
  canvas: ClipCanvasBox
  safeArea: ClipCanvasBox
  /** The owner's own copy of this cut's footage, where the session holds one. */
  frameUrl?: string
  frameFingerprint?: string
  frameStartMs: number
  /** Falls back to the unexpired retained original when the session has no
   *  local copy — the owner may have come back to a project days later
   *  (CLIP-50, CLIP-57). */
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  change: (edit: TimelineEdit, group?: string) => void
  disabled?: boolean
}) {
  const { t } = useTranslation('clips')
  const stage = useRef<HTMLDivElement>(null)
  const grab = useRef<{ x: number; y: number; originX: number; originY: number }>(null)
  const [dragging, setDragging] = useState(false)
  const [live, setLive] = useState<{ x: number; y: number }>()
  // The retained original is remembered WITH the source it belongs to, so a
  // caption on another cut never shows the frame of the last one.
  const [retained, setRetained] = useState<{ fingerprint: string; url: string }>()
  useEffect(() => {
    let active = true
    if (frameUrl || !frameFingerprint || !resolvePlayback) return
    resolvePlayback(frameFingerprint).then(
      (url) => {
        if (active) setRetained({ fingerprint: frameFingerprint, url })
      },
      () => {},
    )
    return () => {
      active = false
    }
  }, [frameUrl, frameFingerprint, resolvePlayback])
  const ground =
    frameUrl || (retained && retained.fingerprint === frameFingerprint ? retained.url : '')
  const box = fragment?.box
  // Automatic placement is a STARTING POINT: the caption is shown where the
  // layout put it until the owner moves it, and their value replaces it from
  // then on (CLIP-15, CDS-38).
  const placed = live ?? text.ownerPosition ?? (box ? { x: box.x, y: box.y } : undefined)
  const percent = (value: number, total: number) => `${(value / Math.max(1, total)) * 100}%`
  const commit = (next: { x: number; y: number }) => {
    if (!box) return
    const at = clampToSafeArea(next.x, next.y, box, safeArea)
    change({ type: 'text', id: text.instanceId, patch: { ownerPosition: at } }, undefined)
    setLive(undefined)
  }
  const move = (dx: number, dy: number) => {
    if (!box || !placed) return
    commit({ x: placed.x + dx, y: placed.y + dy })
  }
  return (
    <div className="space-y-2">
      <Typography variant="fieldTitle">{t('placement.title')}</Typography>
      <div
        ref={stage}
        className="bg-surface-recessed relative w-full overflow-hidden rounded-md"
        style={{ aspectRatio: `${canvas.width} / ${canvas.height}` }}
      >
        {ground ? (
          <video
            muted
            playsInline
            preload="metadata"
            aria-hidden="true"
            tabIndex={-1}
            src={`${ground}#t=${frameStartMs / 1000}`}
            className="absolute inset-0 h-full w-full object-cover"
          />
        ) : (
          <Typography
            variant="meta"
            className="absolute inset-0 flex items-center justify-center p-4 text-center"
          >
            {t('placement.noFrame')}
          </Typography>
        )}
        <svg
          viewBox={`0 0 ${canvas.width} ${canvas.height}`}
          className="pointer-events-none absolute inset-0 h-full w-full"
          aria-hidden="true"
        >
          {dragging && (
            <rect
              x={safeArea.x}
              y={safeArea.y}
              width={safeArea.width}
              height={safeArea.height}
              fill="none"
              stroke="currentColor"
              strokeDasharray="16 12"
              strokeWidth={4}
              data-testid="clip-caption-safe-area"
            />
          )}
          {fragment && placed && (
            <g
              transform={`translate(${placed.x},${placed.y})`}
              /* The server's own drawing of this caption, escaped and validated
                 where it was built: there is no other way to place the very SVG
                 the renderer produces (CDS-83). */
              dangerouslySetInnerHTML={{ __html: fragment.svg }}
            />
          )}
        </svg>
        {fragment && box && placed && (
          <button
            type="button"
            disabled={disabled}
            aria-label={t('placement.handle')}
            className="absolute cursor-move rounded-sm"
            style={{
              left: percent(placed.x, canvas.width),
              top: percent(placed.y, canvas.height),
              width: percent(box.width, canvas.width),
              height: percent(box.height, canvas.height),
            }}
            onPointerDown={(event) => {
              if (disabled) return
              // Keeps the drag with this element when the pointer outruns it.
              event.currentTarget.setPointerCapture?.(event.pointerId)
              grab.current = {
                x: event.clientX,
                y: event.clientY,
                originX: placed.x,
                originY: placed.y,
              }
              setDragging(true)
            }}
            onPointerMove={(event) => {
              const from = grab.current
              const rect = stage.current?.getBoundingClientRect()
              if (!from || !rect || rect.width <= 0) return
              // A drag is not a save path: the position rides local state and
              // the draft queue hears about it once, on release.
              const scale = canvas.width / rect.width
              setLive(
                clampToSafeArea(
                  from.originX + (event.clientX - from.x) * scale,
                  from.originY + (event.clientY - from.y) * scale,
                  box,
                  safeArea,
                ),
              )
            }}
            onPointerUp={() => {
              grab.current = null
              setDragging(false)
              if (live) commit(live)
            }}
            onPointerCancel={() => {
              grab.current = null
              setDragging(false)
              setLive(undefined)
            }}
            onKeyDown={(event) => {
              const step = event.shiftKey
                ? CLIP_CAPTION_PLACEMENT.coarseNudgePx
                : CLIP_CAPTION_PLACEMENT.nudgePx
              const by: Record<string, [number, number]> = {
                ArrowLeft: [-step, 0],
                ArrowRight: [step, 0],
                ArrowUp: [0, -step],
                ArrowDown: [0, step],
              }
              const delta = by[event.key]
              if (!delta || disabled) return
              event.preventDefault()
              move(delta[0], delta[1])
            }}
          />
        )}
      </div>
      {fragment?.representativeFrame && (
        <Typography variant="meta">{t('placement.representative')}</Typography>
      )}
      {!fragment && ground && <Typography variant="meta">{t('placement.drawing')}</Typography>}
      <ClipNoticeList
        notices={notices.filter(
          (n) =>
            n.elementId === text.elementId && n.cutId === text.cutId && n.code.includes('contrast'),
        )}
        language={language}
      />
    </div>
  )
}
