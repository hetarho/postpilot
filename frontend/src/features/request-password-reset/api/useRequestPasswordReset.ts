import { useMutation } from '@connectrpc/connect-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

export function useRequestPasswordReset() {
  const mutation = useMutation(AuthService.method.requestPasswordReset)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
