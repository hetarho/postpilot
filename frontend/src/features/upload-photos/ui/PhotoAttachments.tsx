import { useMemo, useState } from 'react'
import type { PostImage } from '@/entities/image'
import type { PostVideo } from '@/entities/video'
import { AttachmentViewer, postAttachments } from '@/entities/post'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Notice } from '@/shared/ui'
import type { UploadBatchState } from '../model/upload-batch'
import { PhotoDropZone } from './PhotoDropZone'
import { PhotoPicker } from './PhotoPicker'
import { PhotoStrip } from './PhotoStrip'
import { SkippedList } from './SkippedList'
import { UploadProgress } from './UploadProgress'

export interface PhotoAttachmentsUpload extends UploadBatchState {
  addFiles: (files: File[]) => unknown
  retry: (id: string) => void
  dismiss: (id: string) => void
  /** The post the photos need is still being created (a new draft's first pick). */
  creatingPost?: boolean
}

export interface PhotoAttachmentsProps {
  /** The saved photos, in their order. */
  images: readonly PostImage[]
  /** The saved clips, shown after the photos (VIDEO-7). */
  videos?: readonly PostVideo[]
  /** The upload batch feeding this slot: a post's (`useUploadPhotos`) or a keyed one. */
  upload: PhotoAttachmentsUpload
  /** A surface that takes photos alone offers no clip in the picker. */
  photosOnly?: boolean
  /** Shows the attachments without add, delete, rotate or move controls. */
  readOnly?: boolean
  /** Failures of this slot's own actions (creating the post, rotating, moving), each a notice. */
  failures?: readonly (AppFailure | undefined)[]
  onDelete: (image: PostImage) => void
  onRotate?: (image: PostImage) => void
  rotatingId?: string
  onDeleteVideo?: (video: PostVideo) => void
  deletingId?: string
  deleteFailedId?: string
  deleteFailure?: AppFailure
  onMove?: (from: number, to: number) => void
}

/** The one photo slot (POST-100): pick or drop, watch the batch convert and upload, see every photo
 *  as a tile, delete, turn and move it, and press a saved one to see it large. The writing editor and
 *  every other surface that takes photos use this slot rather than a picker and a count of their
 *  own; a surface leaves out the actions it does not support. */
export function PhotoAttachments({
  images,
  videos = [],
  upload,
  photosOnly,
  readOnly = false,
  failures = [],
  onDelete,
  onRotate,
  rotatingId,
  onDeleteVideo,
  deletingId,
  deleteFailedId,
  deleteFailure,
  onMove,
}: PhotoAttachmentsProps) {
  // The large view walks the strip's order — photos, then clips — over the ones with a picture.
  const attachments = useMemo(
    () => postAttachments({ images: [...images], videos: [...videos] }),
    [images, videos],
  )
  const viewOrder = useMemo(
    () => [...attachments.values()].filter((item) => item.viewUrl).map((item) => item.filename),
    [attachments],
  )
  const [viewing, setViewing] = useState<string | null>(null)
  const disabled = readOnly || !!upload.creatingPost
  const addFiles = (files: File[]) => void upload.addFiles(files)

  return (
    <PhotoDropZone onFiles={addFiles} disabled={disabled}>
      <div data-slot="photos" className="flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <PhotoPicker onFiles={addFiles} disabled={disabled} photosOnly={photosOnly} />
          <UploadProgress
            items={upload.items}
            completed={upload.completed}
            creatingPost={upload.creatingPost}
          />
        </div>
        {/* The THEME-18 notice contract, through the primitive: explanatory copy the user has to
          act on is never metadata-sized (THEME-19). */}
        {failures.map(
          (failure, index) =>
            failure && (
              <Notice key={index} tone="danger" role="alert">
                <AppFailureMessage failure={failure} />
              </Notice>
            ),
        )}
        <PhotoStrip
          images={images}
          videos={videos}
          items={upload.items}
          onDelete={onDelete}
          onRotate={onRotate}
          rotatingId={rotatingId}
          onDeleteVideo={onDeleteVideo}
          deletingId={deletingId}
          deleteFailedId={deleteFailedId}
          deleteFailure={deleteFailure}
          onRetry={upload.retry}
          onDismiss={upload.dismiss}
          readOnly={readOnly}
          onView={setViewing}
          onMove={onMove}
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
