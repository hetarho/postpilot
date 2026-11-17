import { useCallback, useMemo, useSyncExternalStore } from 'react'
import type { createUploadHandshake, PostImage } from '@/entities/image'
import { createUploadPipeline } from '../api/upload-pipeline'
import {
  type UploadBatchState,
  peekUploadState,
  subscribeUploadBatch,
  uploadBatch,
} from './upload-batch'

export interface UseKeyedPhotoUploadsArgs {
  /** The batch key: anything that is not a post slug, such as `test-photos:<seed>`. */
  batchKey: string
  /** A handshake answering in the post photo handshake's shapes. */
  handshake: ReturnType<typeof createUploadHandshake>
  /** Filenames already held under this key, for de-duplication (POST-33). */
  taken: readonly string[]
  /** How many photos the key already holds, for the photo ceiling (POST-31). */
  held: number
  onConfirmed: (image: PostImage) => void
}

/** The same gate → convert → PUT → confirm batch as a post's photos, keyed by something that is
 *  not a post and taking photos alone: no post is minted and no post cache is patched. */
export function useKeyedPhotoUploads({
  batchKey,
  handshake,
  taken,
  held,
  onConfirmed,
}: UseKeyedPhotoUploadsArgs): UploadBatchState & {
  addFiles: (files: File[]) => void
  retry: (id: string) => void
  dismiss: (id: string) => void
} {
  const deps = useMemo(
    () => ({
      pipeline: createUploadPipeline(handshake),
      onConfirmed: (_key: string, image: PostImage) => onConfirmed(image),
    }),
    [handshake, onConfirmed],
  )
  const state = useSyncExternalStore(
    useCallback((onChange: () => void) => subscribeUploadBatch(batchKey, onChange), [batchKey]),
    () => peekUploadState(batchKey),
  )
  const options = { photosOnly: true }
  return {
    ...state,
    addFiles: (files) => {
      if (files.length)
        uploadBatch(batchKey, deps, { photos: held, videos: 0 }, options).add(files, taken)
    },
    retry: (id) => uploadBatch(batchKey, deps, undefined, options).retry(id),
    dismiss: (id) => uploadBatch(batchKey, deps, undefined, options).dismiss(id),
  }
}
