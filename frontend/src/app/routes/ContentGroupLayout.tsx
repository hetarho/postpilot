import { Outlet } from '@tanstack/react-router'

/** The route node a content group hangs from. It draws no chrome of its own any more: both
 *  navigation levels belong to `AuthenticatedLayout`, which reads the open group off
 *  `DESTINATIONS` (owner decision 2026-09-22). What is left is the column every group's pages sit
 *  in, kept as a component so the route tree still names the group it is.
 *
 *  It was the second level for a while — a row above the content below the desk, an inner rail
 *  beside it from `lg:` — and the shell deliberately did not know which group was current. One
 *  sidebar needs that fact in the shell, and a group's row belongs in the chrome rather than on
 *  the page, so the knowledge moved up and the chrome moved with it. */
export function ContentGroupLayout() {
  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <Outlet />
    </div>
  )
}
