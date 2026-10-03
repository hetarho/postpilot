import { isPublished, type PostDraft } from '@/entities/post'
import {
  PhotoDropZone,
  PhotoPicker,
  PhotoStrip,
  SkippedList,
  UploadProgress,
  useDeletePhoto,
  useRotatePhoto,
  useUploadPhotos,
} from '@/features/upload-photos'
import { AppFailureMessage, Notice } from '@/shared/ui'

interface EditorPhotosProps {
  post: PostDraft | undefined
  ensureSlug: () => Promise<string>
}

/** The editor's photo slot: pick (or drop), watch them convert and upload, delete. A published
 *  post's photos and clips are shown, and none is added or deleted (POST-86). */
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

  return (
    <PhotoDropZone
      onFiles={(files) => void upload.addFiles(files)}
      disabled={published || upload.creatingPost}
    >
      <div data-slot="photos" className="mt-5 flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <PhotoPicker
            onFiles={(files) => void upload.addFiles(files)}
            disabled={published || upload.creatingPost}
          />
          <UploadProgress
            items={upload.items}
            completed={upload.completed}
            creatingPost={upload.creatingPost}
          />
        </div>
        {/* The THEME-18 notice contract, through the primitive: this was an inlined copy of it at 12px,
          and explanatory copy the user has to act on is never metadata-sized (THEME-19). */}
        {upload.createFailure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={upload.createFailure} />
          </Notice>
        )}
        {rotateFailure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={rotateFailure} />
          </Notice>
        )}
        <PhotoStrip
          images={images}
          videos={videos}
          items={upload.items}
          onDelete={deletePhoto}
          onRotate={rotatePhoto}
          rotatingId={rotatingId}
          onDeleteVideo={deleteVideo}
          deletingId={deletingId}
          deleteFailedId={failedId}
          deleteFailure={deleteFailure}
          onRetry={upload.retry}
          onDismiss={upload.dismiss}
          readOnly={published}
        />
        <SkippedList items={upload.items} onDismiss={upload.dismiss} />
      </div>
    </PhotoDropZone>
  )
}
