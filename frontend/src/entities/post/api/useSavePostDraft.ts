import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { GetPostResponseSchema, PostService } from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

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
      queryClient.setQueryData(
        getPostQueryKey(transport, post.slug),
        create(GetPostResponseSchema, { post }),
      )

      // The list is ordered by updated_at and shows the title, so every save changes it.
      // Marking it stale is enough — the list is on another route, and react-query
      // refetches an inactive query when it is next mounted rather than now.
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
  })
}
