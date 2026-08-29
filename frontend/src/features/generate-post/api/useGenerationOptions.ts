import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { getPostQueryKey, listPostsQueryKey } from '@/entities/post'
import { PostService } from '@/shared/api'

export function useGenerationOptions() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.savePostGenerationOptions)
  return {
    ...mutation,
    save: async (slug: string, targetLength?: number) => {
      const response = await mutation.mutateAsync({ slug, targetLength })
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, slug) }),
        queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) }),
      ])
      return response
    },
  }
}
