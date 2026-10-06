import { Outlet } from '@tanstack/react-router'

/** A pathless route group preserves the existing protected URLs without adding navigation. */
export function ContentGroupLayout() {
  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <Outlet />
    </div>
  )
}
