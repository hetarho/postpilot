import { Outlet } from '@tanstack/react-router'

/** The outermost shell. Deliberately bare: routes reached without a session (/login)
 *  render with no app chrome, and the signed-in chrome belongs to AuthenticatedLayout. */
export function RootLayout() {
  return <Outlet />
}
