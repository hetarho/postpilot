// Shared harness for tests that need a fake AuthService.
//
// Everything is built per test: the app's own transport and QueryClient are module
// singletons, and connect-query keys the cache by transport identity, so reusing them
// would leak one test's session into the next.
import type { ReactNode } from 'react'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import {
  AuthService,
  ChangePasswordResponseSchema,
  GetMeResponseSchema,
  ProtoPlan,
  LoginResponseSchema,
  SignInWithGoogleResponseSchema,
  LogoutResponseSchema,
  RegisterEmailResponseSchema,
  RequestPasswordResetResponseSchema,
  ResendVerificationResponseSchema,
  ResetPasswordResponseSchema,
  SignupResponseSchema,
  VerifyEmailResponseSchema,
} from '@/shared/api'
import { type FakePostsOptions, registerPostService } from './posts'
import { type FakeProvidersOptions, registerProviderService } from './providers'
import { type FakeJobsOptions, registerGenerationService } from './jobs'
import { type FakeVoiceOptions, registerVoiceService } from './voice'
import { type FakeExperimentsOptions, registerExperimentService } from './experiments'
import { type FakePublishingOptions, registerPublishingService } from './publishing'
import { type FakeGuidelinesOptions, registerGuidelineService } from './guidelines'
import { type FakeTemplatesOptions, registerTemplateService } from './templates'
import { type FakeModelCatalogOptions, registerModelCatalogService } from './model-catalog'
import { type FakePlansOptions, registerPlanServices } from './plans'
import { type FakeBillingOptions, registerBillingService } from './billing'
import { connectAppError } from './app-error'

export interface FakeAuthOptions {
  /** The account GetMe reports. `undefined` makes GetMe answer 401, like a real server
   *  with no session.
   *
   *  `plan` defaults to `master` for the same reason migration 0013 backfills existing
   *  accounts to it: every test written before the ladder existed assumed an account with
   *  full authority, and that is what those accounts became. A test about a gated surface
   *  sets a lower tier explicitly. */
  user?: {
    id: string
    plan?: ProtoPlan
    email?: string
    emailVerified?: boolean
    hasPassword?: boolean
  }
  /** Makes Login answer 401, like wrong credentials. */
  loginFails?: boolean
  /** Makes Logout fail, like an API that went away mid-session. */
  logoutFails?: boolean
  signupFails?: boolean
  verificationFails?: boolean
  registerEmailFails?: boolean
  resetPasswordFails?: boolean
  changePasswordFails?: 'wrong-current' | 'not-set'
  googleFailure?: 'GOOGLE_EMAIL_UNVERIFIED' | 'GOOGLE_ACCOUNT_MISMATCH' | 'GOOGLE_SIGNIN_DISABLED'
  tooManyAttempts?: 'login' | 'signup' | 'resend' | 'reset_request' | 'reset' | 'google'
  /** Records every procedure the transport was asked for. */
  calls?: string[]
  /** The PostService the signed-in screens call. Present by default (with no posts) so a
   *  routing test that lands on /posts is not reading an "unimplemented" error instead of
   *  the failure it is actually looking for. */
  posts?: FakePostsOptions
  /** The ProviderService (model catalog). Present by default with an empty registry. */
  providers?: FakeProvidersOptions
  /** The durable job service used by pages that mount active-job polling. */
  jobs?: FakeJobsOptions
  /** The acting account's voice profile and sample mutations. */
  voice?: FakeVoiceOptions
  experiments?: FakeExperimentsOptions
  publishing?: FakePublishingOptions
  /** The acting account's 템플릿 briefs. Present by default with none, so every screen that
   *  mounts the selector reads an empty directory rather than an "unimplemented" error. */
  templates?: FakeTemplatesOptions
  /** The acting account's 작문 지침. Present by default with none, so a screen that mounts the
   *  list reads an empty one rather than an "unimplemented" error. */
  guidelines?: FakeGuidelinesOptions
  /** The plan ladder: the caller's own tier and usage, and the operator's account list. */
  plans?: FakePlansOptions
  billing?: FakeBillingOptions
  /** The operator's model catalog. Present by default with nothing curated and nothing
   *  offered, so a routing test that lands on /admin/models reads an empty catalog rather
   *  than an "unimplemented" error. */
  modelCatalog?: FakeModelCatalogOptions
}

/** A fake backend plus the controls a test needs over it. */
export interface FakeAuthBackend {
  transport: ReturnType<typeof createRouterTransport>
  /** Ends the session server-side, the way an expiry or a logout elsewhere would.
   *  Without it a test cannot model a session that dies while the user is working. */
  expireSession: () => void
}

export function createFakeAuthBackend(options: FakeAuthOptions = {}): FakeAuthBackend {
  const {
    user,
    loginFails,
    logoutFails,
    signupFails,
    verificationFails,
    registerEmailFails,
    resetPasswordFails,
    changePasswordFails,
    googleFailure,
    tooManyAttempts,
    calls,
  } = options
  let session = user

  const transport = createRouterTransport((router) => {
    const { rpc } = router
    rpc(AuthService.method.getMe, () => {
      calls?.push('GetMe')
      if (!session) throw connectAppError('AUTH_REQUIRED', Code.Unauthenticated)
      return create(GetMeResponseSchema, {
        user: {
          id: session.id,
          email: session.email ?? '',
          emailVerified: session.emailVerified ?? false,
          hasPassword: session.hasPassword ?? true,
        },
        plan: session.plan ?? ProtoPlan.MASTER,
      })
    })
    rpc(AuthService.method.login, (req) => {
      calls?.push('Login')
      if (tooManyAttempts === 'login') {
        throw connectAppError('TOO_MANY_ATTEMPTS', Code.ResourceExhausted, {
          retry_at: '2026-09-30T15:00:00Z',
        })
      }
      if (loginFails) throw connectAppError('INVALID_CREDENTIALS', Code.Unauthenticated)
      session = {
        id: req.loginId,
        plan: user?.plan,
        email: user?.email,
        emailVerified: user?.emailVerified,
        hasPassword: user?.hasPassword,
      }
      return create(LoginResponseSchema, {
        user: {
          id: session.id,
          email: session.email ?? '',
          emailVerified: session.emailVerified ?? false,
          hasPassword: session.hasPassword ?? true,
        },
        plan: session.plan ?? ProtoPlan.MASTER,
      })
    })
    rpc(AuthService.method.signInWithGoogle, () => {
      calls?.push('SignInWithGoogle')
      if (tooManyAttempts === 'google') {
        throw connectAppError('TOO_MANY_ATTEMPTS', Code.ResourceExhausted, {
          retry_at: '2026-09-30T15:00:00Z',
        })
      }
      if (googleFailure) throw connectAppError(googleFailure, Code.FailedPrecondition)
      session = {
        id: user?.id ?? 'google@example.com',
        plan: user?.plan,
        email: user?.email ?? 'google@example.com',
        emailVerified: true,
        hasPassword: user?.hasPassword ?? false,
      }
      return create(SignInWithGoogleResponseSchema, {
        user: {
          id: session.id,
          email: session.email,
          emailVerified: true,
          hasPassword: session.hasPassword,
        },
        plan: session.plan ?? ProtoPlan.MASTER,
      })
    })
    rpc(AuthService.method.logout, () => {
      calls?.push('Logout')
      if (logoutFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
      session = undefined
      return create(LogoutResponseSchema, {})
    })
    rpc(AuthService.method.signup, () => {
      calls?.push('Signup')
      if (tooManyAttempts === 'signup') {
        throw connectAppError('TOO_MANY_ATTEMPTS', Code.ResourceExhausted, {
          retry_at: '2026-09-30T15:00:00Z',
        })
      }
      if (signupFails) throw connectAppError('UNKNOWN_FAILURE', Code.Internal)
      return create(SignupResponseSchema, {})
    })
    rpc(AuthService.method.resendVerification, () => {
      calls?.push('ResendVerification')
      if (tooManyAttempts === 'resend') {
        throw connectAppError('TOO_MANY_ATTEMPTS', Code.ResourceExhausted, {
          retry_at: '2026-09-30T15:00:00Z',
        })
      }
      return create(ResendVerificationResponseSchema, {})
    })
    rpc(AuthService.method.verifyEmail, () => {
      calls?.push('VerifyEmail')
      if (verificationFails) {
        throw connectAppError('VERIFICATION_LINK_INVALID', Code.FailedPrecondition)
      }
      return create(VerifyEmailResponseSchema, {})
    })
    rpc(AuthService.method.requestPasswordReset, () => {
      calls?.push('RequestPasswordReset')
      if (tooManyAttempts === 'reset_request') {
        throw connectAppError('TOO_MANY_ATTEMPTS', Code.ResourceExhausted, {
          retry_at: '2026-09-30T15:00:00Z',
        })
      }
      return create(RequestPasswordResetResponseSchema, {})
    })
    rpc(AuthService.method.resetPassword, () => {
      calls?.push('ResetPassword')
      if (tooManyAttempts === 'reset') {
        throw connectAppError('TOO_MANY_ATTEMPTS', Code.ResourceExhausted, {
          retry_at: '2026-09-30T15:00:00Z',
        })
      }
      if (resetPasswordFails) {
        throw connectAppError('RESET_LINK_INVALID', Code.FailedPrecondition)
      }
      return create(ResetPasswordResponseSchema, {})
    })
    rpc(AuthService.method.registerEmail, (request) => {
      calls?.push('RegisterEmail')
      if (registerEmailFails) {
        throw connectAppError('EMAIL_ALREADY_VERIFIED', Code.FailedPrecondition)
      }
      if (session) session = { ...session, email: request.email, emailVerified: false }
      return create(RegisterEmailResponseSchema, {})
    })
    rpc(AuthService.method.changePassword, () => {
      calls?.push('ChangePassword')
      if (changePasswordFails === 'wrong-current') {
        throw connectAppError('CURRENT_PASSWORD_WRONG', Code.FailedPrecondition)
      }
      if (changePasswordFails === 'not-set') {
        throw connectAppError('PASSWORD_NOT_SET', Code.FailedPrecondition)
      }
      session = undefined
      return create(ChangePasswordResponseSchema, {})
    })
    registerPostService(router, { calls, ...options.posts })
    registerProviderService(router, { calls, ...options.providers })
    registerGenerationService(router, { calls, ...options.jobs })
    registerVoiceService(router, { calls, ...options.voice })
    registerExperimentService(router, { calls, ...options.experiments })
    registerPublishingService(router, { calls, ...options.publishing })
    registerTemplateService(router, { calls, ...options.templates })
    registerGuidelineService(router, { calls, ...options.guidelines })
    registerPlanServices(router, { plan: user?.plan, calls, ...options.plans })
    registerBillingService(router, { calls, ...options.billing })
    registerModelCatalogService(router, { calls, ...options.modelCatalog })
  })

  return {
    transport,
    expireSession: () => {
      session = undefined
    },
  }
}

/** The common case: a fake backend when the test never needs to expire the session. */
export function createFakeAuthTransport(options: FakeAuthOptions = {}) {
  return createFakeAuthBackend(options).transport
}

/** A QueryClient with retries off — a 401 is an answer, and a retry would double every
 *  assertion about how many requests were made. */
export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
}

/** Provider order mirrors App.tsx: TransportProvider outside QueryClientProvider. */
export function withProviders(transport: FakeAuthBackend['transport'], queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </TransportProvider>
    )
  }
}
