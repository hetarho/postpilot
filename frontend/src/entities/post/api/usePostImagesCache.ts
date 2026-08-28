import { useMemo } from 'react'
import { clone } from '@bufbuild/protobuf'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { type PostImage, toProtoImage } from '@/entities/image/@x/post'
import { type GetPostResponse, GetPostResponseSchema, type Post } from '@/shared/api'
import { getPostQueryKey } from './post-queries'

/** Edits the photos of a cached `GetPost` answer, in place.
 *
 *  Patching rather than invalidating is deliberate: a refetch would mint fresh presigned
 *  URLs for every photo, and every thumbnail on screen would download again — on
 *  cellular, after each of eight uploads. The server is still the source of truth; the
 *  patch only mirrors what a successful confirm or delete has just told us.
 *
 *  The slug is an argument rather than a hook parameter because the caller may learn it
 *  late: the first photo picked in a new draft creates the post, and the confirm lands
 *  under a slug the editor did not have when it rendered. */
export function usePostImagesCache(): {
  append: (slug: string, image: PostImage) => void
  remove: (slug: string, imageId: string) => void
} {
  const queryClient = useQueryClient()
  const transport = useTransport()

  // Stable for the life of the clients, so a consumer can hand these to something that
  // outlives its render (the upload batch) without rebinding on every render.
  return useMemo(() => {
    const update = (slug: string, edit: (post: Post) => void) => {
      queryClient.setQueryData<GetPostResponse>(getPostQueryKey(transport, slug), (old) => {
        if (!old?.post) return old
        const next = clone(GetPostResponseSchema, old)
        if (next.post) edit(next.post)
        return next
      })
    }

    return {
      append: (slug: string, image: PostImage) =>
        update(slug, (post) => {
          // Idempotent: a confirm retried after a lost response returns the same photo.
          if (post.images.some((existing) => existing.id === image.id)) return
          post.images.push(toProtoImage(image))
        }),
      remove: (slug: string, imageId: string) =>
        update(slug, (post) => {
          post.images = post.images.filter((existing) => existing.id !== imageId)
        }),
    }
  }, [queryClient, transport])
}
