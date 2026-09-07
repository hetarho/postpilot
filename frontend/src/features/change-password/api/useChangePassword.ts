import { useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'

export function useChangePassword() {
  const queryClient = useQueryClient()
  const mutation = useMutation(AuthService.method.changePassword, {
    onSuccess: () => queryClient.removeQueries(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
