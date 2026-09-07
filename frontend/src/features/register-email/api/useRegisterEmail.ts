import { useMutation } from '@connectrpc/connect-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

export function useRegisterEmail() {
  const mutation = useMutation(AuthService.method.registerEmail)
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
