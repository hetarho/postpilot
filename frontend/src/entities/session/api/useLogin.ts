import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, AuthService, GetMeResponseSchema } from '@/shared/api'
import { getMeQueryKey } from './session-queries'

/** Logs in and seeds the session cache from the response.
 *
 *  Seeding matters for more than a saved round trip: the route guard reads this entry,
 *  so without it the navigation that follows a successful login would find no session
 *  and bounce straight back to /login. */
export function useLogin() {
  const queryClient = useQueryClient()
  // The transport the hooks are mounted on — the same one the key must be built from.
  const transport = useTransport()

  const mutation = useMutation(AuthService.method.login, {
    onSuccess: (data) => {
      // setQueryData is typed to the query's own data, so this has to be a real
      // protobuf message, not a plain object literal.
      // The tier is seeded with the id: the shell's first paint gates master-only surfaces
      // on it, and a seed that carried only the id would render them wrong for one refetch.
      queryClient.setQueryData(
        getMeQueryKey(transport),
        create(GetMeResponseSchema, { user: data.user, plan: data.plan }),
      )
    },
  })

  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
