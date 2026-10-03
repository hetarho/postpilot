import { type PostImage, toPostImage, useRotateImage } from '@/entities/image'
import { usePostImagesCache } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { nextQuarterTurn } from '@/shared/lib'

/** Turns a photo a quarter clockwise from the strip (POST-107): the RPC, then the cached post
 *  takes the photo as the server answers it, so every surface shows the new turn at once. */
export function useRotatePhoto(slug: string | undefined): {
  rotatePhoto: (image: PostImage) => void
  /** The photo whose turn is in flight, so the strip can hold that one control. */
  rotatingId: string | undefined
  /** Why the last turn was refused. */
  failure: AppFailure | undefined
} {
  const rotate = useRotateImage()
  const cache = usePostImagesCache()
  return {
    rotatingId: rotate.isPending ? rotate.variables?.imageId : undefined,
    failure: rotate.error ? appFailureFromConnect(rotate.error) : undefined,
    rotatePhoto: (image) => {
      if (!slug) return
      rotate.mutate(
        { imageId: image.id, rotation: nextQuarterTurn(image.rotation) },
        {
          onSuccess: (response) => {
            if (response.image) cache.replace(slug, toPostImage(response.image))
          },
        },
      )
    },
  }
}
