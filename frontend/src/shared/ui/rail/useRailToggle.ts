import { useState } from 'react'

export interface RailToggle {
  open: boolean
  toggle: () => void
}

/** Whether a rail is showing its names or only its glyphs. Nothing else in the shell needs to
 *  know, so it is a plain piece of component state rather than anything the router carries. */
export function useRailToggle(initiallyOpen = true): RailToggle {
  const [open, setOpen] = useState(initiallyOpen)
  return { open, toggle: () => setOpen((current) => !current) }
}
