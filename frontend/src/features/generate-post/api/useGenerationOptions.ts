import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { getPostQueryKey, listPostsQueryKey } from '@/entities/post'
import { PostService } from '@/shared/api'

/** The two per-post run options that are validated numbers rather than choices (POST-20, POST-63).
 *  `targetLength` undefined means natural length; `tagCount` is always a number. */
export interface GenerationOptionValues {
  targetLength?: number
  tagCount: number
}

export function useGenerationOptions() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.savePostGenerationOptions)
  return {
    ...mutation,
    save: async (slug: string, values: GenerationOptionValues) => {
      const response = await mutation.mutateAsync({
        slug,
        targetLength: values.targetLength,
        tagCount: values.tagCount,
      })
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, slug) }),
        queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) }),
      ])
      return response
    },
  }
}
