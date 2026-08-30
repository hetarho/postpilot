import { clone, create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
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
    super('다른 화면에서 글이 바뀌었어요. 최신 글을 불러온 뒤 다시 수정해 주세요.')
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
        if (ConnectError.from(cause).code === Code.Aborted) throw new ContentRevisionConflictError()
        throw cause
      }
    },
  }
}
