import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { blogFieldToProto } from '@/entities/blog-field/@x/post'
import { qualityMetricToProto } from '@/entities/quality/@x/post'
import { PostService } from '@/shared/api'
import type { GenerationOptionsSet } from '../model/types'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

/** The writing brief's one save (POST-89). One request carries the whole set — natural length as
 *  an absent `targetLength`, 없음 as a present UNSPECIFIED `field` — so no save can keep a stale
 *  member or drop one it forgot. */
export function useGenerationOptions() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.savePostGenerationOptions)
  return {
    ...mutation,
    save: async (slug: string, set: GenerationOptionsSet) => {
      const response = await mutation.mutateAsync({
        slug,
        targetLength: set.targetLength,
        tagCount: set.tagCount,
        useMemory: set.useMemory,
        qualityRules: { metrics: set.qualityRules.map(qualityMetricToProto) },
        field: blogFieldToProto(set.field),
      })
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, slug) }),
        queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) }),
      ])
      return response
    },
  }
}
