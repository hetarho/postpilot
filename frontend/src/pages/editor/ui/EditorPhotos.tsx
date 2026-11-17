import { isPublished, type PostDraft } from '@/entities/post'
import {
  PhotoAttachments,
  useDeletePhoto,
  useReorderPhotos,
  useRotatePhoto,
  useUploadPhotos,
} from '@/features/upload-photos'

interface EditorPhotosProps {
  post: PostDraft | undefined
  ensureSlug: () => Promise<string>
}

/** The editor's photo slot: the shared `PhotoAttachments` bound to the post — pick (or drop), watch
 *  them convert and upload, delete, turn, move, and press a saved one to see it large (POST-100). A
 *  published post's photos and clips are shown and open large, and none is added or deleted
 *  (POST-86). */
export function EditorPhotos({ post, ensureSlug }: EditorPhotosProps) {
  const slug = post?.slug
  const published = post ? isPublished(post) : false
  const images = post?.images ?? []
  const videos = post?.videos ?? []
  const upload = useUploadPhotos({
    slug,
    // ONE filename namespace across both kinds (VIDEO-5), so both lists are handed over.
    taken: [...images.map((image) => image.filename), ...videos.map((video) => video.filename)],
    held: { photos: images.length, videos: videos.length },
    ensureSlug,
  })
  const {
    deletePhoto,
    deleteVideo,
    deletingId,
    failedId,
    failure: deleteFailure,
  } = useDeletePhoto(slug)
  const { rotatePhoto, rotatingId, failure: rotateFailure } = useRotatePhoto(slug)
  const { movePhoto, failure: moveFailure } = useReorderPhotos(slug, images)

  return (
    <div className="mt-4">
      <PhotoAttachments
        images={images}
        videos={videos}
        upload={upload}
        readOnly={published}
        failures={[upload.createFailure, rotateFailure, moveFailure]}
        onDelete={deletePhoto}
        onRotate={rotatePhoto}
        rotatingId={rotatingId}
        onDeleteVideo={deleteVideo}
        deletingId={deletingId}
        deleteFailedId={failedId}
        deleteFailure={deleteFailure}
        onMove={movePhoto}
      />
    </div>
  )
}
