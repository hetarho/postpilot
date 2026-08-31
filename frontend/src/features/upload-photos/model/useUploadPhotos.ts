import { useCallback, useMemo, useState, useSyncExternalStore } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { usePostImagesCache } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { createUploadPipeline } from '../api/upload-pipeline'
import {
  type UploadBatchState,
  type UploadItem,
  partitionFiles,
  peekUploadState,
  subscribeUploadBatch,
  uploadBatch,
} from './upload-batch'

export interface UseUploadPhotosArgs {
  /** The post the photos attach to, or undefined for a draft with no slug yet. */
  slug: string | undefined
  /** Filenames the post already holds, for de-duplication. */
  taken: readonly string[]
  /** Produces the slug, creating the post first if needed — the editor's autosave owns
   *  that, so it is injected. */
  ensureSlug: () => Promise<string>
}

/** The React end of the upload batch (`upload-batch.ts`). */
export function useUploadPhotos({
  slug,
  taken,
  ensureSlug,
}: UseUploadPhotosArgs): UploadBatchState & {
  addFiles: (files: File[]) => Promise<void>
  retry: (id: string) => void
  dismiss: (id: string) => void
  /** True while the post the photos need is being created (a new draft's first pick).
   *  The picker should be disabled: a second pick would attach the same files twice. */
  creatingPost: boolean
  /** Structured reason why the post could not be created; selected files were dropped. */
  createFailure: AppFailure | undefined
} {
  const transport = useTransport()
  const cache = usePostImagesCache()
  const deps = useMemo(
    () => ({ pipeline: createUploadPipeline(transport), onConfirmed: cache.append }),
    [transport, cache.append],
  )
  const [creatingPost, setCreatingPost] = useState(false)
  const [createFailure, setCreateFailure] = useState<AppFailure>()
  // Skipped files from a pick that had nothing to upload: there is no post to hang a
  // batch on, and none should be created just to list them.
  const [skippedWithoutPost, setSkippedWithoutPost] = useState<readonly UploadItem[]>([])

  const state = useSyncExternalStore(
    useCallback(
      (onChange: () => void) => (slug ? subscribeUploadBatch(slug, onChange) : () => {}),
      [slug],
    ),
    () => peekUploadState(slug),
  )
  const items = useMemo(
    () => (skippedWithoutPost.length ? [...skippedWithoutPost, ...state.items] : state.items),
    [skippedWithoutPost, state.items],
  )

  return {
    items,
    completed: state.completed,
    creatingPost,
    createFailure,
    addFiles: async (files) => {
      if (files.length === 0 || creatingPost) return
      if (slug) {
        uploadBatch(slug, deps).add(files, taken)
        return
      }
      const { accepted, skipped } = partitionFiles(files)
      if (accepted.length === 0) {
        setSkippedWithoutPost((previous) => [...previous, ...skipped])
        return
      }
      // For a new draft this creates the post and moves the URL; the batch is keyed by
      // the minted slug, so the editor that navigation mounts picks it up — skipped files
      // included, so the list survives the move.
      setCreatingPost(true)
      setCreateFailure(undefined)
      try {
        const target = await ensureSlug()
        uploadBatch(target, deps).add(files, taken)
      } catch (cause) {
        setCreateFailure(appFailureFromConnect(cause))
      } finally {
        setCreatingPost(false)
      }
    },
    retry: (id) => {
      if (slug) uploadBatch(slug, deps).retry(id)
    },
    dismiss: (id) => {
      if (skippedWithoutPost.some((item) => item.id === id)) {
        setSkippedWithoutPost((previous) => previous.filter((item) => item.id !== id))
        return
      }
      if (slug) uploadBatch(slug, deps).dismiss(id)
    },
  }
}
