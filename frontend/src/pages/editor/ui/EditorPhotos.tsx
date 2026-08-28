import type { PostDraft } from '@/entities/post'
import {
  PhotoPicker,
  PhotoStrip,
  SkippedList,
  UploadProgress,
  useDeletePhoto,
  useUploadPhotos,
} from '@/features/upload-photos'

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
      {upload.createFailed && (
        <p
          role="alert"
          className="bg-notice-danger-bg text-notice-danger-fg rounded-md px-3 py-2 text-xs"
        >
          사진을 붙일 글을 만들지 못했어요. 다시 시도해 주세요.
        </p>
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
