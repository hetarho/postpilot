import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, GetPostResponseSchema, PostService } from '@/shared/api'
import type { PostDraft } from '../model/types'
import { getPostQueryKey, listPostsQueryKey, toPostDraft } from './post-queries'

/** Records, replaces or clears ('') the post's Naver Blog address (POST-73, POST-75). The answer
 *  IS the post — published, still published, or back to finalized — so the detail entry is
 *  replaced rather than invalidated, and the list is marked stale for its badge. */
export function useSavePublishedUrl() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.savePostPublishedUrl, {
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
    save: async (slug: string, url: string): Promise<PostDraft> => {
      const response = await mutation.mutateAsync({ slug, url })
      if (!response.post) throw new Error('SavePostPublishedUrl returned no post')
      return toPostDraft(response.post)
    },
    pending: mutation.isPending,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    reset: mutation.reset,
  }
}
