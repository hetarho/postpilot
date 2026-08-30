import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  type GetPostResponse,
  GetPostResponseSchema,
  type Post,
  PostSchema,
  PostService,
  PurposeRefSchema,
  VoiceRefSchema,
} from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

/** Applies only the fields this mutation owns to the cached post.
 *
 *  SavePostDraft answers with a whole post snapshot, but the request is in flight while
 *  uploads and generation independently advance images, observations, active_job and
 *  content. Installing that snapshot wholesale could roll any of them back in the cache.
 *  Title, memo and the two assignments are the fields this mutation settles; every other
 *  field remains owned by GetPost or its focused mutation patch.
 *
 *  A reassignment is the one save that also moves the machine baseline: the server clears
 *  it in the same write (spec/policy/posts.md), and it refuses to reassign while a job could
 *  advance that baseline, so mirroring the cleared fields cannot roll a job's result back. */
export function applyingSavedDraft(saved: Post, cached: GetPostResponse | undefined): Post {
  if (!cached?.post) return saved
  const post = clone(PostSchema, cached.post)
  post.title = saved.title
  post.memo = saved.memo
  if (saved.voice) {
    if (saved.voice.id !== cached.post.voice?.id) {
      post.machineBaselineRevision = saved.machineBaselineRevision
      post.machineBaselineVoiceId = saved.machineBaselineVoiceId
      post.canFinalize = saved.canFinalize
    }
    post.voice = clone(VoiceRefSchema, saved.voice)
  }
  // Unconditional, unlike the voice: the response always reports the current 용도, and an
  // unset one is a real answer (없음). A `if (saved.purpose)` guard would make a clear
  // invisible until the next GetPost.
  post.purpose = saved.purpose ? clone(PurposeRefSchema, saved.purpose) : undefined
  return post
}

/** Create-or-update for a draft: an empty slug creates the post and returns the minted
 *  one (spec/policy/posts.md). This is the autosave endpoint, so it is called about once
 *  a second while someone types. */
export function useSavePostDraft() {
  const queryClient = useQueryClient()
  // The transport the hooks are mounted on — the same one the keys must be built from.
  const transport = useTransport()

  return useMutation(PostService.method.savePostDraft, {
    onSuccess: (data) => {
      const post = data.post
      if (!post) return

      // Seeding matters for more than a saved round trip: the first save of a new post
      // moves the URL to the minted slug, and the editor that route mounts reads this
      // entry. Without it the user would watch the text they just typed disappear and
      // come back.
      const key = getPostQueryKey(transport, post.slug)
      queryClient.setQueryData(
        key,
        create(GetPostResponseSchema, {
          post: applyingSavedDraft(post, queryClient.getQueryData<GetPostResponse>(key)),
        }),
      )

      // The list is ordered by updated_at and shows the title, so every save changes it.
      // Marking it stale is enough — the list is on another route, and react-query
      // refetches an inactive query when it is next mounted rather than now.
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
  })
}
