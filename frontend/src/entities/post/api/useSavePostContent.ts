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
  ReplacementCandidateSchema,
  type PostContent,
} from '@/shared/api'
import type { ReplacementCandidate } from '../model/replacements'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { toReplacementCandidates } from './replacement-mappers'

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
      // A take spent its candidate in this very save (POST-79), so the screen reads the server's
      // shorter list without a refetch.
      post.replacementCandidates = saved.replacementCandidates.map((candidate) =>
        clone(ReplacementCandidateSchema, candidate),
      )
      queryClient.setQueryData(key, create(GetPostResponseSchema, { post }))
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
    /** Saves the content at `expectedRevision`, spending the candidates `takenCandidates` names
     *  (indices into the list at that revision). Answers the new revision and the list left. */
    save: async (
      slug: string,
      content: PostContent,
      expectedRevision: bigint,
      takenCandidates: readonly number[] = [],
    ): Promise<{ revision: bigint; candidates: ReplacementCandidate[] }> => {
      try {
        const response = await mutation.mutateAsync({
          slug,
          content: create(PostContentSchema, content),
          expectedRevision,
          takenCandidates: [...takenCandidates],
        })
        if (!response.post) throw new Error('SavePostContent returned no post')
        return {
          revision: response.post.contentRevision,
          candidates: toReplacementCandidates(response.post.replacementCandidates),
        }
      } catch (cause) {
        if (appFailureFromConnect(cause).reason === 'POST_CONTENT_STALE') {
          throw new ContentRevisionConflictError()
        }
        throw cause
      }
    },
  }
}
