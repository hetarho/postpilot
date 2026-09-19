import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { GetPostResponseSchema, PostService } from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

/** Finalizing is a post write: it fixes the revision the post is published from, and the answer
 *  IS the post, so the detail entry is replaced rather than invalidated. */
export function useFinalizePost() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.finalizePost, {
    onSuccess: (response) => {
      if (!response.post) return
      queryClient.setQueryData(
        getPostQueryKey(transport, response.post.slug),
        create(GetPostResponseSchema, { post: response.post }),
      )
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
  })
  return {
    ...mutation,
    finalize: async (slug: string, expectedRevision: bigint) => {
      const response = await mutation.mutateAsync({ slug, expectedRevision })
      if (!response.post) throw new Error('FinalizePost returned no post')
      return response.post
    },
  }
}
