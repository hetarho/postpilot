import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  type GetPostResponse,
  GetPostResponseSchema,
  type Post,
  PostSchema,
  PostService,
} from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

/** The saved post, but with the photo list the cache already holds.
 *
 *  A save answers with the whole post, photos included, and is in flight about once a
 *  second while someone types. Taking its photo list would race the confirm and delete
 *  patches (`usePostImagesCache`): a save that left before a delete lands after it and
 *  puts the photo back. It would also replace every presigned URL on screen with a fresh
 *  one each second, and the browser would download every thumbnail again. The text is
 *  what a save settles; the photos are settled by `GetPost` and by the patches. */
function keepingCachedImages(saved: Post, cached: GetPostResponse | undefined): Post {
  if (!cached?.post) return saved
  const post = clone(PostSchema, saved)
  post.images = cached.post.images
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
          post: keepingCachedImages(post, queryClient.getQueryData<GetPostResponse>(key)),
        }),
      )

      // The list is ordered by updated_at and shows the title, so every save changes it.
      // Marking it stale is enough — the list is on another route, and react-query
      // refetches an inactive query when it is next mounted rather than now.
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
  })
}
