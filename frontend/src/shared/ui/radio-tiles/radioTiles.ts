import { clsx } from 'clsx'
import type { KeyboardEvent } from 'react'

const STEP: Record<string, number> = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }

/** Choice tiles as one WAI-APG radio group, so a slice keeps its own title, grid and tile
 *  content: spread `group` on the element holding every tile and `tile(index, layout)` on each
 *  tile's button. Only the selected tile is tabbable (none when `selected` is -1) and every
 *  arrow moves focus and choice together, wrapping at either end. */
export function radioTiles(count: number, selected: number, select: (index: number) => void) {
  return {
    group: {
      role: 'radiogroup',
      onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
        const step = STEP[event.key]
        if (!step) return
        event.preventDefault()
        const next = (selected + step + count) % count
        select(next)
        event.currentTarget.querySelectorAll<HTMLElement>('[role="radio"]')[next]?.focus()
      },
    },
    tile: (index: number, layout: string) => ({
      type: 'button' as const,
      role: 'radio',
      'aria-checked': index === selected,
      tabIndex: index === selected ? 0 : -1,
      onClick: () => select(index),
      className: clsx(
        'flex min-h-11 min-w-11 flex-col rounded-md p-2 text-left transition-colors',
        layout,
        index === selected
          ? 'bg-row-bg-active text-content-primary ring-stroke-accent ring-2'
          : 'text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active',
      ),
    }),
  }
}
