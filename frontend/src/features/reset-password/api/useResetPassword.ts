import { useMutation } from '@connectrpc/connect-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

export function useResetPassword() {
  const mutation = useMutation(AuthService.method.resetPassword)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
