import { useEffect, useRef, type PointerEvent, type RefObject } from 'react'
import { applyTimelineEdit, type ClipEditPlan, type TimelineEdit } from '@/entities/clip-plan'
import {
  cutGestureEdit,
  timelinePoint,
  type CutGesture,
  type SourceBounds,
} from './timeline-gesture'

export function useTimelineGesture({
  canvas,
  plan,
  timeMs,
  onSeek,
  onCommit,
  onPreview,
  bounds,
}: {
  canvas: RefObject<HTMLDivElement | null>
  plan: ClipEditPlan
  timeMs: number
  onSeek?: (ms: number) => void
  onCommit?: (edit: TimelineEdit) => void
  onPreview?: (plan?: ClipEditPlan) => void
  bounds?: (id: string) => SourceBounds
}) {
  const drag = useRef<{
    pointer: number
    x: number
    left: number
    width: number
    plan: ClipEditPlan
    timeMs: number
    id?: string
    intent?: CutGesture
    edit?: TimelineEdit
  }>(undefined)
  const cancel = () => {
    const prior = drag.current
    drag.current = undefined
    onPreview?.()
    if (prior && !prior.intent) onSeek?.(prior.timeMs)
  }
  useEffect(() => {
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && drag.current) {
        event.preventDefault()
        event.stopPropagation()
        cancel()
      }
    }
    document.addEventListener('keydown', escape, true)
    return () => {
      document.removeEventListener('keydown', escape, true)
    }
  })
  const begin = (event: PointerEvent<HTMLElement>, id?: string, intent?: CutGesture) => {
    if (event.button !== 0 || (intent ? !onCommit : !onSeek) || !canvas.current) return
    const rect = canvas.current.getBoundingClientRect()
    if (!(rect.width > 0)) return
    event.preventDefault()
    event.stopPropagation()
    event.currentTarget.setPointerCapture?.(event.pointerId)
    drag.current = {
      pointer: event.pointerId,
      x: event.clientX,
      left: rect.left,
      width: rect.width,
      plan,
      timeMs,
      id,
      intent,
    }
  }
  const move = (event: PointerEvent<HTMLElement>) => {
    const active = drag.current
    if (!active || active.pointer !== event.pointerId) return
    event.preventDefault()
    const position = timelinePoint(event.clientX, active.left, active.width, active.plan.durationMs)
    if (!active.intent) {
      onSeek?.(position)
      return
    }
    const edit = cutGestureEdit(
      active.plan,
      active.id!,
      active.intent,
      ((event.clientX - active.x) / active.width) * active.plan.durationMs,
      position,
      bounds?.(active.id!) ?? { startMs: 0, endMs: 0 },
    )
    active.edit = edit
    onPreview?.(edit ? applyTimelineEdit(active.plan, edit) : undefined)
  }
  const end = (event: PointerEvent<HTMLElement>) => {
    if (drag.current?.pointer !== event.pointerId) return
    const edit = drag.current.edit
    drag.current = undefined
    onPreview?.()
    if (edit) onCommit?.(edit)
    event.currentTarget.releasePointerCapture?.(event.pointerId)
  }
  const seekAt = (x: number) => {
    const rect = canvas.current?.getBoundingClientRect()
    if (rect) onSeek?.(timelinePoint(x, rect.left, rect.width, plan.durationMs))
  }
  return { begin, move, end, cancel, seekAt }
}
