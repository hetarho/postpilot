import { type PostImage, useDeleteImage } from '@/entities/image'
import { type PostVideo, useDeleteVideo } from '@/entities/video'
import { usePostImagesCache } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

/** Deletes an attachment from the strip: the RPC, then the cached post loses the entry.
 *
 *  Both kinds are here rather than in two hooks because the strip has ONE delete affordance
 *  and one confirm sheet: which RPC runs is the attachment's business, not the caller's. */
export function useDeletePhoto(slug: string | undefined): {
  deletePhoto: (image: PostImage) => void
  deleteVideo: (video: PostVideo) => void
  /** The photo whose delete is in flight, so the strip can dim exactly that one. */
  deletingId: string | undefined
  /** The photo whose last delete failed. */
  failedId: string | undefined
  /** The allowlisted reason returned for that failed delete. */
  failure: AppFailure | undefined
} {
  const deleteImage = useDeleteImage()
  const deleteVideoMutation = useDeleteVideo()
  const cache = usePostImagesCache()

  const pendingId = deleteImage.isPending
    ? deleteImage.variables?.imageId
    : deleteVideoMutation.isPending
      ? deleteVideoMutation.variables?.videoId
      : undefined
  const erroredId = deleteImage.isError
    ? deleteImage.variables?.imageId
    : deleteVideoMutation.isError
      ? deleteVideoMutation.variables?.videoId
      : undefined
  const error = deleteImage.error ?? deleteVideoMutation.error

  return {
    deletingId: pendingId,
    failedId: erroredId,
    failure: error ? appFailureFromConnect(error) : undefined,
    deletePhoto: (image) => {
      if (!slug) return
      deleteImage.mutate({ imageId: image.id }, { onSuccess: () => cache.remove(slug, image.id) })
    },
    deleteVideo: (video) => {
      if (!slug) return
      deleteVideoMutation.mutate(
        { videoId: video.id },
        { onSuccess: () => cache.removeVideo(slug, video.id) },
      )
    },
  }
}
