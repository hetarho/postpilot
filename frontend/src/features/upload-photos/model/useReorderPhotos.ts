import { type PostImage, useReorderImages } from '@/entities/image'
import { usePostImagesCache, useRefreshPostImages } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

/** Moves a photo in the post's order from the strip (POST-116). The cached post takes the new
 *  order at once, so every surface follows the move before the answer; a refusal — a photo added
 *  or deleted since the post was read — reads the post again, which drops the local order. */
export function useReorderPhotos(
  slug: string | undefined,
  images: readonly PostImage[],
): {
  movePhoto: (from: number, to: number) => void
  /** Why the last move was refused. */
  failure: AppFailure | undefined
} {
  const cache = usePostImagesCache()
  const refresh = useRefreshPostImages(slug ?? '')
  const reorder = useReorderImages()
  return {
    failure: reorder.error ? appFailureFromConnect(reorder.error) : undefined,
    movePhoto: (from, to) => {
      if (!slug || from === to || !images[from] || !images[to]) return
      const ids = images.map((image) => image.id)
      const [moved] = ids.splice(from, 1)
      ids.splice(to, 0, moved)
      cache.reorder(slug, ids)
      reorder.mutate({ postSlug: slug, imageIds: ids }, { onError: refresh })
    },
  }
}
