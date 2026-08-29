import type { PostDraft } from '@/entities/post'
import {
  PhotoPicker,
  PhotoStrip,
  SkippedList,
  UploadProgress,
  useDeletePhoto,
  useUploadPhotos,
} from '@/features/upload-photos'
import { Notice } from '@/shared/ui'

interface EditorPhotosProps {
  post: PostDraft | undefined
  ensureSlug: () => Promise<string>
}

/** The editor's photo slot: pick, watch them convert and upload, delete. */
export function EditorPhotos({ post, ensureSlug }: EditorPhotosProps) {
  const slug = post?.slug
  const images = post?.images ?? []
  const upload = useUploadPhotos({
    slug,
    taken: images.map((image) => image.filename),
    ensureSlug,
  })
  const { deletePhoto, deletingId, failedId } = useDeletePhoto(slug)

  return (
    <section data-slot="photos" className="mt-5 flex flex-col gap-3">
      <div className="flex items-center gap-3">
        <PhotoPicker
          onFiles={(files) => void upload.addFiles(files)}
          disabled={upload.creatingPost}
        />
        <UploadProgress
          items={upload.items}
          completed={upload.completed}
          creatingPost={upload.creatingPost}
        />
      </div>
      {/* The §2.6 notice contract, through the primitive: this was an inlined copy of it at 12px,
          and explanatory copy the user has to act on is never metadata-sized (§3). */}
      {upload.createFailed && (
        <Notice tone="danger" role="alert">
          사진을 붙일 글을 만들지 못했어요. 다시 시도해 주세요.
        </Notice>
      )}
      <PhotoStrip
        images={images}
        items={upload.items}
        onDelete={deletePhoto}
        deletingId={deletingId}
        deleteFailedId={failedId}
        onRetry={upload.retry}
        onDismiss={upload.dismiss}
      />
      <SkippedList items={upload.items} onDismiss={upload.dismiss} />
    </section>
  )
}
