import { useMutation } from '@connectrpc/connect-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

export function useSignUp() {
  const mutation = useMutation(AuthService.method.signup)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
