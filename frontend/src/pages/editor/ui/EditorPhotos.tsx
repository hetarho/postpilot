import { useMemo, useState } from 'react'
import { AttachmentViewer, isPublished, postAttachments, type PostDraft } from '@/entities/post'
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

/** The editor's photo slot: pick (or drop), watch them convert and upload, delete, and press a
 *  saved one to see it large (POST-100). A published post's photos and clips are shown and open
 *  large, and none is added or deleted (POST-86). */
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
  // The large view walks the strip's order — photos, then clips — over the ones with a picture.
  const attachments = useMemo(
    () => postAttachments({ images: post?.images ?? [], videos: post?.videos ?? [] }),
    [post?.images, post?.videos],
  )
  const viewOrder = useMemo(
    () => [...attachments.values()].filter((item) => item.viewUrl).map((item) => item.filename),
    [attachments],
  )
  const [viewing, setViewing] = useState<string | null>(null)

  return (
    <PhotoDropZone
      onFiles={(files) => void upload.addFiles(files)}
      disabled={published || upload.creatingPost}
    >
      <div data-slot="photos" className="mt-4 flex flex-col gap-3">
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
          onView={setViewing}
        />
        <SkippedList items={upload.items} onDismiss={upload.dismiss} />
        <AttachmentViewer
          files={viewOrder}
          attachments={attachments}
          viewing={viewing}
          onView={setViewing}
          onClose={() => setViewing(null)}
        />
      </div>
    </PhotoDropZone>
  )
}
