import { forwardRef, type ComponentPropsWithoutRef } from 'react'

/** Renders the attributes and recipe returned by radioTiles without altering its roving behavior. */
export const RadioTile = forwardRef<HTMLButtonElement, ComponentPropsWithoutRef<'button'>>(
  function RadioTile({ type = 'button', ...props }, ref) {
    return <button ref={ref} type={type} {...props} />
  },
)
