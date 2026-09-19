import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { forgotPasswordSearchSchema } from '@/pages/forgot-password'
import { googleSignInCallbackSearchSchema } from '@/pages/google-sign-in-callback'
import { loginSearchSchema } from '@/pages/login'
import { resetPasswordSearchSchema } from '@/pages/reset-password'
import { signupSearchSchema } from '@/pages/signup'
import { verifyEmailSearchSchema } from '@/pages/verify-email'
import { LoginPage } from '@/pages/login'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import { hasActivePublicSession } from './publicGuard'
import { rootRoute } from './tree'

export const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: loginSearchSchema,
  beforeLoad: async ({ context, search }) => {
    // Resolve first, then throw. A `throw redirect()` inside a try block would be caught
    // by that block's own catch and silently swallowed.
    //
    // Unlike the guard below, this route swallows an outage: the login form is what a
    // user reaches for when the app is misbehaving, so it must render even when the API
    // cannot answer at all. Submitting will then fail with a real message.
    const signedIn = await hasActivePublicSession(context)

    if (signedIn) {
      throw redirect({
        to: isInAppPath(search.redirect) ? search.redirect : SIGNED_IN_HOME,
        replace: true,
      })
    }
  },
  component: LoginPage,
})

export const signupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/signup',
  validateSearch: signupSearchSchema,
  beforeLoad: async ({ context, search }) => {
    if (await hasActivePublicSession(context)) {
      throw redirect({
        to: isInAppPath(search.redirect) ? search.redirect : SIGNED_IN_HOME,
        replace: true,
      })
    }
  },
  component: lazyRouteComponent(() => import('@/pages/signup'), 'SignupPage'),
})

// The OAuth provider lands here before a Postpilot session exists. It is therefore a
// direct public child with no reverse/session guard; the page validates its per-tab state.
export const googleSignInCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login/google/callback',
  validateSearch: googleSignInCallbackSearchSchema,
  component: lazyRouteComponent(
    () => import('@/pages/google-sign-in-callback'),
    'GoogleSignInCallbackPage',
  ),
})

export const verifyEmailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/verify-email',
  validateSearch: verifyEmailSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/verify-email'), 'VerifyEmailPage'),
})

export const forgotPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/forgot-password',
  validateSearch: forgotPasswordSearchSchema,
  beforeLoad: async ({ context, search }) => {
    if (await hasActivePublicSession(context)) {
      throw redirect({
        to: isInAppPath(search.redirect) ? search.redirect : SIGNED_IN_HOME,
        replace: true,
      })
    }
  },
  component: lazyRouteComponent(() => import('@/pages/forgot-password'), 'ForgotPasswordPage'),
})

export const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reset-password',
  validateSearch: resetPasswordSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/reset-password'), 'ResetPasswordPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const authRoutes = [
  loginRoute,
  signupRoute,
  googleSignInCallbackRoute,
  verifyEmailRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
]
