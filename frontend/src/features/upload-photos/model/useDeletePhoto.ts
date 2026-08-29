import { type PostImage, useDeleteImage } from '@/entities/image'
import { usePostImagesCache } from '@/entities/post'

/** Deletes a photo from the strip: the RPC, then the cached post loses the entry. */
export function useDeletePhoto(slug: string | undefined): {
  deletePhoto: (image: PostImage) => void
  /** The photo whose delete is in flight, so the strip can dim exactly that one. */
  deletingId: string | undefined
  /** The photo whose last delete failed. */
  failedId: string | undefined
} {
  const deleteImage = useDeleteImage()
  const cache = usePostImagesCache()

  return {
    deletingId: deleteImage.isPending ? deleteImage.variables?.imageId : undefined,
    failedId: deleteImage.isError ? deleteImage.variables?.imageId : undefined,
    deletePhoto: (image) => {
      if (!slug) return
      deleteImage.mutate({ imageId: image.id }, { onSuccess: () => cache.remove(slug, image.id) })
    },
  }
}
