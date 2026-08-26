import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { HomePage } from '@/pages/home'
import { RootLayout } from './RootLayout'

// Code-based routing. The app layer owns the routes; screen UI is delegated to the
// pages layer (FSD).
const rootRoute = createRootRoute({ component: RootLayout })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
})

const routeTree = rootRoute.addChildren([indexRoute])

export const router = createRouter({ routeTree })

// Register the router instance for type safety across the app (Link, useNavigate, …).
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
