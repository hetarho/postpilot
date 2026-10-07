import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { invalidateQuality } from '@/entities/quality/@x/post'
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
import { isFinalizedOrLater, isPublished } from '../model/types'

export class ContentRevisionConflictError extends Error {
  constructor() {
    super(i18next.t('POST_CONTENT_STALE', { ns: 'errors' }))
  }
}

export function useSavePostContent() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(PostService.method.savePostContent, {
    onMutate: (input) => ({
      query: queryClient.getQueryCache().find({
        queryKey: getPostQueryKey(transport, input.slug ?? ''),
        exact: true,
      }),
    }),
    onSuccess: (data, input, scope) => {
      const saved = data.post
      if (!saved) return
      const key = getPostQueryKey(transport, saved.slug)
      if (queryClient.getQueryCache().find({ queryKey: key, exact: true }) !== scope?.query) return
      const cached = queryClient.getQueryData<GetPostResponse>(key)
      // A refetch or machine replacement can overtake this response. The old save is still
      // acknowledged to its queue, but must never replace a newer result or its evidence.
      if (cached?.post && cached.post.contentRevision > saved.contentRevision) return
      if (
        cached?.post &&
        cached.post.contentRevision === saved.contentRevision &&
        ((isPublished(cached.post) && !isPublished(saved)) ||
          (isFinalizedOrLater(cached.post) && !isFinalizedOrLater(saved)))
      )
        return
      const post = cached?.post ? clone(PostSchema, cached.post) : clone(PostSchema, saved)
      post.content = saved.content ? clone(PostContentSchema, saved.content) : undefined
      post.contentRevision = saved.contentRevision
      post.contentHash = saved.contentHash
      // Input/source withdrawal can happen while the content save is in flight. Do not revive
      // available frozen sources from its earlier snapshot; a fresh owning read aligns them.
      const sourcesChanged = Boolean(
        cached?.post && cached.post.inputRevision > saved.inputRevision,
      )
      post.contentOrigins = sourcesChanged ? undefined : saved.contentOrigins
      post.contentLanguage = saved.contentLanguage
      post.machineBaselineRevision = saved.machineBaselineRevision
      post.canFinalize = saved.canFinalize
      post.status = saved.status
      post.finalizedRevision = saved.finalizedRevision
      post.finalizedAt = saved.finalizedAt
      post.updatedAt = saved.updatedAt
      queryClient.setQueryData(key, create(GetPostResponseSchema, { post }))
      if (sourcesChanged)
        void queryClient.invalidateQueries({
          queryKey: getPostQueryKey(transport, input.slug ?? ''),
        })
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
      // A new revision is a new measurement (QUAL-3); the row reads it by revision already, and
      // this reaches every other reading that counted the old text.
      void invalidateQuality(queryClient, transport)
    },
    // Published elsewhere (POST-86): the refetch reads the post locked, which unmounts ②'s
    // editor and its queue.
    onError: (error, variables) => {
      if (!variables.slug || appFailureFromConnect(error).reason !== 'POST_PUBLISHED_LOCKED') return
      void queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, variables.slug) })
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
