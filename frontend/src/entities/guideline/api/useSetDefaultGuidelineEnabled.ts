import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { GuidelineService } from '@/shared/api'
import type { DefaultGuidelineEntry, GuidelineKind } from '../model/types'
import { guidelineErrorMessage } from './guideline-errors'
import { fromGuidelineKind, guidelineKindQueryKey } from './guideline-queries'

interface WithDefaults {
  defaults: DefaultGuidelineEntry[]
}

/** A 기본 지침's switch (GUIDE-43). It saves on change and applies from the next run: nothing
 *  already queued changes, because a run freezes its guidelines at the start.
 *
 *  Optimistic, because a switch that waits a round trip before it moves reads as broken: the
 *  cached row flips at once, a refusal puts it back and says why, and the list is re-read either
 *  way so the screen ends on what the server holds. */
export function useSetDefaultGuidelineEnabled(ownerId: string, kind: GuidelineKind) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const queryKey = guidelineKindQueryKey(transport, ownerId, kind)
  const mutation = useMutation(GuidelineService.method.setDefaultGuidelineEnabled, {
    onMutate: async (request) => {
      await queryClient.cancelQueries({ queryKey, exact: true })
      const previous = queryClient.getQueryData<WithDefaults>(queryKey)
      if (previous) {
        queryClient.setQueryData<WithDefaults>(queryKey, {
          ...previous,
          defaults: previous.defaults.map((entry) =>
            entry.key === request.key ? { ...entry, enabled: request.enabled ?? false } : entry,
          ),
        })
      }
      return { previous }
    },
    onError: (_error, _request, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous)
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey, exact: true }),
  })
  return {
    ...mutation,
    errorMessage: guidelineErrorMessage(mutation.error),
    setEnabled: (key: string, enabled: boolean) =>
      mutation.mutateAsync({ kind: fromGuidelineKind(kind), key, enabled }),
  }
}
