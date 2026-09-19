import { useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

/** The account's own credential writes. They live with the session entity rather than in the
 *  action slices because they are plain CRUD over the one noun the entity owns (ARCH-14's verb
 *  line): each is a bare mutation nobody renders, and the forms that do render live in the
 *  features that call them. None of them touches another noun's cache — the one exception is
 *  a password change, which drops every cached answer of the session it just invalidated. */

export function useSignUp() {
  const mutation = useMutation(AuthService.method.signup)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}

export function useVerifyEmail() {
  const mutation = useMutation(AuthService.method.verifyEmail)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}

export function useResendVerification() {
  const mutation = useMutation(AuthService.method.resendVerification)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}

export function useRequestPasswordReset() {
  const mutation = useMutation(AuthService.method.requestPasswordReset)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}

export function useResetPassword() {
  const mutation = useMutation(AuthService.method.resetPassword)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}

export function useRegisterEmail() {
  const mutation = useMutation(AuthService.method.registerEmail)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}

export function useChangePassword() {
  const queryClient = useQueryClient()
  const mutation = useMutation(AuthService.method.changePassword, {
    // The password change ends every other session of the account, so every cached answer was
    // read under credentials that no longer exist.
    onSuccess: () => queryClient.removeQueries(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
