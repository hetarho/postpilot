import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { PostService } from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

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
    /** The memory opt-in on its own (MEM-18, POST-71). Presence is the edit unit on this call, so
     *  sending only the flag cannot disturb the two numbers the brief saves — and it changes no
     *  status, revision or baseline, which is why it autosaves on the toggle. */
    saveUseMemory: async (slug: string, useMemory: boolean) => {
      const response = await mutation.mutateAsync({ slug, useMemory })
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, slug) }),
        queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) }),
      ])
      return response
    },
  }
}
