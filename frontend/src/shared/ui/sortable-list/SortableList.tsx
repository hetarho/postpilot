import { useState, type ReactNode } from 'react'
import { ChevronDown, ChevronUp, GripVertical } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import { Button } from '../button/Button'

export interface SortableItem {
  id: string
  content: ReactNode
}

/** A vertical list whose rows can be reordered (design-language §1.1 — a reorderable list has
 *  no product noun in its name, so it is a primitive and not one slice's private control).
 *
 *  It offers BOTH ways deliberately. Pointer drag is what people expect from a builder, but
 *  HTML5 drag events do not fire on touch, and the base breakpoint here is a 360px phone
 *  (§1.5) — so the move buttons are not a fallback for keyboard users, they are the only way
 *  the primary device can reorder at all. Neither is hidden behind the other.
 *
 *  Rows carry no background of their own, so a `divide-divider` hairline separates them: one
 *  of the four cases §1.3 allows a border in. The drag-over row steps to `surface-raised`
 *  instead of drawing an insertion rule. */
export function SortableList({
  items,
  onReorder,
  labels,
  className,
}: {
  items: readonly SortableItem[]
  /** Moves the item at `from` so it sits at `to`. The caller owns the list. */
  onReorder: (from: number, to: number) => void
  labels: { drag: string; up: string; down: string }
  className?: string
}) {
  const [dragging, setDragging] = useState<number | null>(null)
  const [over, setOver] = useState<number | null>(null)

  const finish = () => {
    setDragging(null)
    setOver(null)
  }

  return (
    <ul className={twMerge('divide-divider divide-y', className)}>
      {items.map((item, index) => (
        <li
          key={item.id}
          draggable
          onDragStart={(event) => {
            setDragging(index)
            // Required for Firefox to start a drag at all; the payload itself is unused
            // because the indices live in state.
            event.dataTransfer.setData('text/plain', item.id)
            event.dataTransfer.effectAllowed = 'move'
          }}
          onDragOver={(event) => {
            if (dragging === null) return
            event.preventDefault()
            setOver(index)
          }}
          onDrop={(event) => {
            event.preventDefault()
            if (dragging !== null && dragging !== index) onReorder(dragging, index)
            finish()
          }}
          onDragEnd={finish}
          className={twMerge(
            'flex items-start gap-2 py-3',
            over === index && dragging !== null && dragging !== index && 'bg-surface-raised',
          )}
        >
          <span
            aria-hidden
            className="text-content-tertiary mt-1 shrink-0 cursor-grab"
            title={labels.drag}
          >
            <GripVertical className="size-5" />
          </span>
          <div className="min-w-0 flex-1">{item.content}</div>
          <div className="flex shrink-0 flex-col">
            <Button
              variant="ghost"
              size="icon"
              aria-label={labels.up}
              disabled={index === 0}
              onClick={() => onReorder(index, index - 1)}
            >
              <ChevronUp className="size-5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label={labels.down}
              disabled={index === items.length - 1}
              onClick={() => onReorder(index, index + 1)}
            >
              <ChevronDown className="size-5" />
            </Button>
          </div>
        </li>
      ))}
    </ul>
  )
}
