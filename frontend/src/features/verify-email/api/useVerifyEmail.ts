import { useMutation } from '@connectrpc/connect-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

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
