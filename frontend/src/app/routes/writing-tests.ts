import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { writingTestSearchSchema, writingTestHistorySearchSchema } from '@/pages/writing-tests'
import { safeInternalPath } from '@/shared/lib/navigation'
import { authenticatedRoute } from './tree'

export const writingTestsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/tests',
  validateSearch: writingTestSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/writing-tests'), 'WritingTestsPage'),
})
export const writingTestHistoryRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/tests/history',
  validateSearch: writingTestHistorySearchSchema,
  component: lazyRouteComponent(() => import('@/pages/writing-tests'), 'WritingTestHistoryPage'),
})
export const writingTestRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/tests/$testId',
  validateSearch: (search): { entry?: string } => ({
    entry:
      typeof search.entry === 'string' && safeInternalPath(search.entry) ? search.entry : undefined,
  }),
  component: lazyRouteComponent(() => import('@/pages/writing-tests'), 'WritingTestPage'),
})
export const writingTestRoutes = [writingTestsRoute, writingTestHistoryRoute, writingTestRoute]
