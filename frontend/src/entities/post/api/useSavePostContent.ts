import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import {
  appFailureFromConnect,
  GetPostResponseSchema,
  type GetPostResponse,
  PostContentSchema,
  PostSchema,
  PostService,
  type PostContent,
} from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

export class ContentRevisionConflictError extends Error {
  constructor() {
    super(i18next.t('POST_CONTENT_STALE', { ns: 'errors' }))
  }
}

export function useSavePostContent() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.savePostContent, {
    onSuccess: (data) => {
      const saved = data.post
      if (!saved) return
      const key = getPostQueryKey(transport, saved.slug)
      const cached = queryClient.getQueryData<GetPostResponse>(key)
      const post = cached?.post ? clone(PostSchema, cached.post) : clone(PostSchema, saved)
      post.content = saved.content ? clone(PostContentSchema, saved.content) : undefined
      post.contentRevision = saved.contentRevision
      post.contentLanguage = saved.contentLanguage
      post.machineBaselineRevision = saved.machineBaselineRevision
      post.canFinalize = saved.canFinalize
      post.status = saved.status
      post.finalizedRevision = saved.finalizedRevision
      post.finalizedAt = saved.finalizedAt
      post.updatedAt = saved.updatedAt
      queryClient.setQueryData(key, create(GetPostResponseSchema, { post }))
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
  })

  return {
    save: async (slug: string, content: PostContent, expectedRevision: bigint) => {
      try {
        const response = await mutation.mutateAsync({
          slug,
          content: create(PostContentSchema, content),
          expectedRevision,
        })
        if (!response.post) throw new Error('SavePostContent returned no post')
        return response.post.contentRevision
      } catch (cause) {
        if (appFailureFromConnect(cause).reason === 'POST_CONTENT_STALE') {
          throw new ContentRevisionConflictError()
        }
        throw cause
      }
    },
  }
}
