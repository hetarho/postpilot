import { useRef } from 'react'
import {
  ANCHORED_PANEL_MAX_VIEWPORT_RATIO,
  ANCHORED_PANEL_MIN_PX,
  ANCHORED_PANEL_TRIGGER_GAP_PX,
  ANCHORED_PANEL_VIEWPORT_GUTTER_PX,
} from './config'
import { useAnchoredDisclosure } from './useAnchoredDisclosure'
import {
  ANCHORED_PANEL_ATTRIBUTE,
  useAnchoredPanel,
  type AnchoredPanelPlacement,
} from './useAnchoredPanel'

const PLACEMENT: AnchoredPanelPlacement = {
  gapPx: ANCHORED_PANEL_TRIGGER_GAP_PX,
  gutterPx: ANCHORED_PANEL_VIEWPORT_GUTTER_PX,
  minPx: ANCHORED_PANEL_MIN_PX,
  maxViewportRatio: ANCHORED_PANEL_MAX_VIEWPORT_RATIO,
}

/** The wiring every content-width anchored panel shares — `InlinePopover`'s and `Toggletip`'s: the
 *  press-or-hover disclosure (THEME-32), the measured placement (THEME-30) and the props the
 *  portalled panel carries. The trigger's props stay with each caller, because they differ.
 *  `enabled: false` keeps the panel from being placed while its disclosure would still be open. */
export function useAnchoredContentPanel<T extends HTMLElement>({
  enabled = true,
}: { enabled?: boolean } = {}) {
  const triggerRef = useRef<T>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const disclosure = useAnchoredDisclosure(triggerRef, panelRef)
  const box = useAnchoredPanel({
    open: enabled && disclosure.open !== '',
    triggerRef,
    panelRef,
    placement: PLACEMENT,
    width: 'content',
    onGone: (focusWasInside) => disclosure.close(focusWasInside, true),
  })
  const panelProps = {
    ref: panelRef,
    [ANCHORED_PANEL_ATTRIBUTE]: '',
    onPointerDown: disclosure.pin,
    onPointerEnter: disclosure.pointerEnter,
    onPointerLeave: disclosure.pointerLeave,
    style: box && { left: box.left, top: box.top, bottom: box.bottom },
  }
  return { triggerRef, panelRef, disclosure, box, panelProps }
}
