import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import {
  writingTestSearchSchema,
  writingTestHistorySearchSchema,
  photoTestSearchSchema,
} from '@/pages/writing-tests'
import { testRecordSearchSchema } from '@/pages/model-experiment'
import { safeInternalPath } from '@/shared/lib/navigation'
import { authenticatedRoute } from './tree'

export const writingTestsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/tests',
  validateSearch: writingTestSearchSchema,
  // Observer models are compared by the photo observation test (MODEL-97), never by writing.
  beforeLoad: ({ search }) => {
    if (search.stage === 'observe')
      throw redirect({
        to: '/tests/photos',
        search: photoTestSearchSchema({ ...search }),
        replace: true,
      })
  },
  component: lazyRouteComponent(() => import('@/pages/writing-tests'), 'WritingTestsPage'),
})
// Declared before /tests/$testId so the literal path wins.
export const photoTestsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/tests/photos',
  validateSearch: photoTestSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/writing-tests'), 'PhotoTestsPage'),
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
export const writingTestRecordRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/tests/records/$id',
  validateSearch: testRecordSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/model-experiment'), 'WritingTestRecordPage'),
})
export const writingTestRoutes = [
  writingTestsRoute,
  writingTestHistoryRoute,
  photoTestsRoute,
  writingTestRecordRoute,
  writingTestRoute,
]
