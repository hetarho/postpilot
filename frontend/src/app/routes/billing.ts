import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { billingCheckoutSearchSchema } from '@/pages/billing-checkout'
import { billingMethodFailSearchSchema } from '@/pages/billing-method-fail'
import { billingMethodSuccessSearchSchema } from '@/pages/billing-method-success'
import { authenticatedRoute } from './tree'

export const plansRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/plans',
  component: lazyRouteComponent(() => import('@/pages/plans'), 'PlansPage'),
})

export const billingRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/billing',
  component: lazyRouteComponent(() => import('@/pages/billing'), 'BillingPage'),
})

export const billingCheckoutRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/billing/checkout',
  validateSearch: billingCheckoutSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/billing-checkout'), 'BillingCheckoutPage'),
})

export const billingMethodSuccessRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/billing/method/success',
  validateSearch: billingMethodSuccessSearchSchema,
  component: lazyRouteComponent(
    () => import('@/pages/billing-method-success'),
    'BillingMethodSuccessPage',
  ),
})

export const billingMethodFailRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/billing/method/fail',
  validateSearch: billingMethodFailSearchSchema,
  component: lazyRouteComponent(
    () => import('@/pages/billing-method-fail'),
    'BillingMethodFailPage',
  ),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const billingRoutes = [
  plansRoute,
  billingRoute,
  billingCheckoutRoute,
  billingMethodSuccessRoute,
  billingMethodFailRoute,
]
