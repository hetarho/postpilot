import { useLayoutEffect, useRef, useState } from 'react'
import { useWindowVirtualizer } from '@tanstack/react-virtual'
import type { AdminCatalogEntry } from '@/entities/model-catalog'
import { CATALOG_ROW_ESTIMATE_PX, CATALOG_ROW_OVERSCAN } from '@/shared/config'
import { CatalogModelRow } from './CatalogModelRow'

/** The catalog list, virtualized.
 *
 *  The provider offers several hundred models and the unfiltered list is all of them, each row
 *  carrying badges and up to two portalled Listboxes. Mounting them all made the screen
 *  unusable — every keystroke in the search box re-rendered hundreds of subtrees — so only the
 *  rows near the viewport exist at any moment.
 *
 *  It virtualizes the WINDOW rather than a scroll container of its own. A nested
 *  `overflow-y-auto` would steal every vertical swipe that lands in it, which design-language
 *  §4.4 forbids on a phone; the page stays the one scroller and the list only reserves its
 *  height. `scrollMargin` is what makes that work: the list starts partway down the page, so the
 *  virtualizer has to be told where its own top edge sits in the document.
 *
 *  Row heights are measured rather than assumed — an enabled row is much taller than a plain
 *  one, and a long Korean label wraps — so `estimateSize` only positions the scrollbar until the
 *  real height is known. */
export function CatalogModelList({ entries }: { entries: readonly AdminCatalogEntry[] }) {
  const listRef = useRef<HTMLUListElement>(null)
  const [listOffset, setListOffset] = useState(0)

  // Measured before paint, and on every render rather than once: the banners above this list
  // appear and disappear, which moves its top edge, and a stale offset puts every row a banner's
  // height out of place. The equality guard is what keeps that from looping — a re-run that
  // measures the same top sets no state, so there is no second render to chain from.
  // eslint-disable-next-line react-hooks/exhaustive-deps -- deliberate: see above
  useLayoutEffect(() => {
    const top = listRef.current?.offsetTop ?? 0
    setListOffset((current) => (current === top ? current : top))
  })

  const virtualizer = useWindowVirtualizer({
    count: entries.length,
    estimateSize: () => CATALOG_ROW_ESTIMATE_PX,
    overscan: CATALOG_ROW_OVERSCAN,
    scrollMargin: listOffset,
    // Keyed by model id, so measured heights follow their row when a filter changes the order
    // instead of being reused by whatever now sits at that index.
    getItemKey: (index) => entries[index]?.modelId ?? index,
    measureElement: (element) => {
      // A visible row is never really zero pixels tall, so a zero here means it has not been
      // laid out yet. Believing it collapses the list's height to nothing, which makes the
      // virtualizer decide it needs every row to fill the viewport — the opposite of the point.
      const measured = element.getBoundingClientRect().height
      return measured > 0 ? measured : CATALOG_ROW_ESTIMATE_PX
    },
  })

  return (
    <ul ref={listRef} className="relative mt-3" style={{ height: virtualizer.getTotalSize() }}>
      {virtualizer.getVirtualItems().map((item) => {
        const entry = entries[item.index]
        if (!entry) return null
        return (
          <li
            key={item.key}
            data-index={item.index}
            ref={virtualizer.measureElement}
            // Absolute placement and a measured offset are values, not utilities: there is no
            // Tailwind step for "wherever this row happens to start". `pb-3` inside the
            // positioned wrapper is the gap between rows, since a virtualized row cannot get one
            // from a grid it is not in.
            className="absolute top-0 left-0 w-full pb-3"
            style={{ transform: `translateY(${item.start - virtualizer.options.scrollMargin}px)` }}
          >
            <CatalogModelRow entry={entry} />
          </li>
        )
      })}
    </ul>
  )
}
