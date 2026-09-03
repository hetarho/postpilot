// The upload pipeline, per post, outside React.
//
// Like the draft queue it outlives the editor on purpose: the first photo of a new draft
// is picked in an editor that the mint navigation is about to unmount, and a batch of
// eight conversions should not stop because the user tapped back to the list. One batch
// per post. A confirmed photo leaves the batch at once — from then on it is the post's
// photo, shown from the cached post like every other.
import type { PostImage } from '@/entities/image'
import { UploadObjectMissing, UploadRejected, UploadRpcFailure } from '@/entities/image'
import type { AppFailure } from '@/shared/api'
import { UPLOAD_CONVERT_CONCURRENCY } from '@/shared/config'
import { DecodeError, dedupeFilename, jpegFilename } from '@/shared/lib'
import { type SkipReason, filterFile } from './filter'

export type UploadStatus =
  'selected' | 'converting' | 'uploading' | 'confirming' | 'skipped' | 'failed'

/** Why an upload stopped. `network` is anything a retry may fix (a failed PUT, an
 *  expired URL, a lost confirm); the others are the server's answer. */
export type UploadFailure = 'network' | 'duplicate-filename' | 'invalid'

export interface UploadItem {
  id: string
  /** The selected file's name, for the skipped list. */
  name: string
  /** What the photo is filed under once uploaded — de-duplicated within the post. */
  filename: string
  status: UploadStatus
  /** Set when `status` is `skipped`. */
  reason?: SkipReason
  /** The converted copy, once conversion succeeds. */
  previewUrl?: string
  /** Set when `status` is `failed`. */
  failure?: UploadFailure
  /** Stable server refusal for an RPC failure. Local decode/PUT failures have none. */
  appFailure?: AppFailure
}

export interface UploadBatchState {
  items: readonly UploadItem[]
  /** Photos confirmed since the batch was last idle — for "올리는 중 n/m". */
  completed: number
}

export interface ConvertedPhoto {
  blob: Blob
  width: number
  height: number
}

/** The four steps of PRD F-2's upload path, injected so this module knows nothing of the
 *  transport, the canvas or the decoder. `createUpload` and `confirm` throw
 *  `UploadRejected` for an answer the server gave; anything else thrown is retryable. */
export interface UploadPipeline {
  convert(file: File): Promise<ConvertedPhoto>
  createUpload(
    slug: string,
    filename: string,
  ): Promise<{ uploadId: string; putUrl: string; contentType: string }>
  put(putUrl: string, contentType: string, blob: Blob): Promise<void>
  confirm(uploadId: string, width: number, height: number): Promise<PostImage>
}

export interface UploadBatchDeps {
  pipeline: UploadPipeline
  /** A photo the server has recorded. Its `viewUrl` is the local preview, since the
   *  confirm answer carries none. */
  onConfirmed: (slug: string, image: PostImage) => void
}

export interface UploadBatchHandle {
  /** Filters, names and queues the files. `taken` is the filenames the post already has. */
  add: (files: File[], taken: Iterable<string>) => void
  /** For a `failed` item: back to `CreateUpload` with a fresh `upload_id`. */
  retry: (id: string) => void
  /** Drops a `skipped` or `failed` item from the list. */
  dismiss: (id: string) => void
}

interface Batch {
  slug: string
  state: UploadBatchState
  /** Kept only until converted, then only until confirmed — the original never lingers. */
  files: Map<string, File>
  converted: Map<string, ConvertedPhoto>
  /** The upload id of an item whose PUT landed but whose confirm did not come back. The
   *  retry confirms that id again (confirm is idempotent) rather than starting over —
   *  starting over would be answered `AlreadyExists` if the confirm had in fact landed. */
  awaitingConfirm: Map<string, string>
  /** Item ids waiting for a conversion slot, in order. */
  convertQueue: string[]
  converting: number
  listeners: Set<() => void>
  deps: UploadBatchDeps
  discarded: boolean
}

const batches = new Map<string, Batch>()
/** Preview URLs handed to the post cache. Nothing else remembers them, and they are
 *  revoked only when the session ends. */
const handedOff = new Set<string>()
let itemSequence = 0

const EMPTY_STATE: UploadBatchState = { items: [], completed: 0 }

function publish(batch: Batch): void {
  for (const listener of batch.listeners) listener()
}

function setItems(
  batch: Batch,
  items: readonly UploadItem[],
  completed = batch.state.completed,
): void {
  batch.state = { items, completed }
  publish(batch)
}

function patch(batch: Batch, id: string, changes: Partial<UploadItem>): void {
  setItems(
    batch,
    batch.state.items.map((item) => (item.id === id ? { ...item, ...changes } : item)),
  )
}

function itemOf(batch: Batch, id: string): UploadItem | undefined {
  return batch.state.items.find((item) => item.id === id)
}

function isActive(item: UploadItem): boolean {
  return item.status !== 'skipped' && item.status !== 'failed'
}

/** Drops a batch nobody is watching and that has nothing to show or do. */
function collect(batch: Batch): void {
  if (batch.state.items.length === 0 && batch.listeners.size === 0) batches.delete(batch.slug)
}

function pumpConversions(batch: Batch): void {
  while (batch.converting < UPLOAD_CONVERT_CONCURRENCY && batch.convertQueue.length > 0) {
    const id = batch.convertQueue.shift()!
    batch.converting += 1
    void convert(batch, id).finally(() => {
      batch.converting -= 1
      pumpConversions(batch)
    })
  }
}

async function convert(batch: Batch, id: string): Promise<void> {
  const file = batch.files.get(id)
  if (batch.discarded || !file) return
  patch(batch, id, { status: 'converting' })
  try {
    const converted = await batch.deps.pipeline.convert(file)
    if (batch.discarded) return
    batch.files.delete(id)
    batch.converted.set(id, converted)
    patch(batch, id, { previewUrl: URL.createObjectURL(converted.blob) })
    void upload(batch, id)
  } catch (error) {
    if (batch.discarded) return
    batch.files.delete(id)
    // A file the device cannot decode is skipped with the reason; the rest proceed
    // (PRD §7).
    const reason: SkipReason = error instanceof DecodeError ? error.reason : 'unreadable'
    patch(batch, id, { status: 'skipped', reason })
  }
}

async function upload(batch: Batch, id: string): Promise<void> {
  const converted = batch.converted.get(id)
  const item = itemOf(batch, id)
  if (batch.discarded || !converted || !item) return
  try {
    let uploadId = batch.awaitingConfirm.get(id)
    if (!uploadId) {
      patch(batch, id, { status: 'uploading', failure: undefined, appFailure: undefined })
      // Never from a kept URL: a fresh `upload_id` each time, and the server replaces the
      // pending upload that held this filename.
      const presigned = await batch.deps.pipeline.createUpload(batch.slug, item.filename)
      // Not a byte after the session ends: a PUT here would file the previous user's
      // photo as an orphan under the next one's post.
      if (batch.discarded) return
      await batch.deps.pipeline.put(presigned.putUrl, presigned.contentType, converted.blob)
      if (batch.discarded) return
      uploadId = presigned.uploadId
      batch.awaitingConfirm.set(id, uploadId)
    }
    patch(batch, id, { status: 'confirming', failure: undefined, appFailure: undefined })
    const image = await batch.deps.pipeline.confirm(uploadId, converted.width, converted.height)
    if (batch.discarded) return
    batch.awaitingConfirm.delete(id)
    finish(batch, id, image)
  } catch (error) {
    if (batch.discarded) return
    // The object is not there, so the PUT is what has to happen again.
    if (error instanceof UploadObjectMissing) batch.awaitingConfirm.delete(id)
    const failure: UploadFailure = error instanceof UploadRejected ? error.reason : 'network'
    const appFailure =
      error instanceof UploadRejected ||
      error instanceof UploadObjectMissing ||
      error instanceof UploadRpcFailure
        ? error.failure
        : undefined
    patch(batch, id, { status: 'failed', failure, appFailure })
  }
}

function finish(batch: Batch, id: string, image: PostImage): void {
  const previewUrl = itemOf(batch, id)?.previewUrl ?? ''
  batch.converted.delete(id)
  if (previewUrl) handedOff.add(previewUrl)
  setItems(
    batch,
    batch.state.items.filter((each) => each.id !== id),
    batch.state.completed + 1,
  )
  batch.deps.onConfirmed(batch.slug, { ...image, viewUrl: image.viewUrl || previewUrl })
  collect(batch)
}

/** The batch for `slug`, created on first use. The deps are replaced on every call, so a
 *  batch that outlived its editor works with the live transport of the next one. */
export function uploadBatch(slug: string, deps: UploadBatchDeps): UploadBatchHandle {
  const attached = batchFor(slug)
  attached.deps = deps

  return {
    add: (files, taken) => {
      const names = new Set(taken)
      for (const item of attached.state.items) {
        if (item.status !== 'skipped') names.add(item.filename)
      }
      const added: UploadItem[] = []
      // `names` already holds every filename the post keeps plus every one this pick has
      // claimed, so its size is exactly what the ceiling has to be measured against.
      for (const file of files) {
        const verdict = filterFile(file, names.size)
        if (verdict.kind === 'skipped') {
          added.push(skippedItem(file, verdict.reason))
          continue
        }
        const id = nextItemId()
        const filename = dedupeFilename(jpegFilename(file.name), names)
        names.add(filename)
        added.push({ id, name: file.name, filename, status: 'selected' })
        attached.files.set(id, file)
      }
      // A new selection on an idle batch starts the count over, so "올리는 중 1/3" is
      // about these three and not about the eight from before.
      const idle = !attached.state.items.some(isActive)
      setItems(attached, [...attached.state.items, ...added], idle ? 0 : attached.state.completed)
      for (const item of added) {
        if (item.status === 'selected') attached.convertQueue.push(item.id)
      }
      pumpConversions(attached)
    },

    retry: (id) => {
      const item = itemOf(attached, id)
      if (item?.status === 'failed') void upload(attached, id)
    },

    dismiss: (id) => {
      const item = itemOf(attached, id)
      if (!item || (item.status !== 'skipped' && item.status !== 'failed')) return
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl)
      attached.files.delete(id)
      attached.converted.delete(id)
      attached.awaitingConfirm.delete(id)
      setItems(
        attached,
        attached.state.items.filter((each) => each.id !== id),
      )
      collect(attached)
    },
  }
}

function nextItemId(): string {
  itemSequence += 1
  return `upload:${itemSequence}`
}

function skippedItem(file: File, reason: SkipReason): UploadItem {
  return { id: nextItemId(), name: file.name, filename: '', status: 'skipped', reason }
}

/** The selection gate, runnable before there is a post to attach to. A pick made of
 *  nothing but skipped files must not create a post just to report them. */
export function partitionFiles(files: File[]): { accepted: File[]; skipped: UploadItem[] } {
  const accepted: File[] = []
  const skipped: UploadItem[] = []
  for (const file of files) {
    // There is no post yet, so the only photos that count toward the ceiling are the ones
    // this same pick has already accepted.
    const verdict = filterFile(file, accepted.length)
    if (verdict.kind === 'skipped') skipped.push(skippedItem(file, verdict.reason))
    else accepted.push(file)
  }
  return { accepted, skipped }
}

function batchFor(slug: string): Batch {
  let batch = batches.get(slug)
  if (!batch) {
    batch = {
      slug,
      state: EMPTY_STATE,
      files: new Map(),
      converted: new Map(),
      awaitingConfirm: new Map(),
      convertQueue: [],
      converting: 0,
      listeners: new Set(),
      deps: {
        pipeline: undefined as unknown as UploadPipeline,
        onConfirmed: () => {},
      },
      discarded: false,
    }
    batches.set(slug, batch)
  }
  return batch
}

/** For `useSyncExternalStore`: a stable snapshot per change. */
export function peekUploadState(slug: string | undefined): UploadBatchState {
  return (slug && batches.get(slug)?.state) || EMPTY_STATE
}

export function subscribeUploadBatch(slug: string, listener: () => void): () => void {
  const batch = batchFor(slug)
  batch.listeners.add(listener)
  return () => {
    batch.listeners.delete(listener)
    collect(batch)
  }
}

/** Drops every batch, abandoning whatever was still converting or uploading.
 *
 *  Same reason as the draft queues: a batch outlives its editor, never its session — a
 *  confirm landing after someone else signed in on this device would file the photo
 *  under the new account's cookie. Called by the app layer on logout and on a session
 *  that died mid-use. */
export function discardUploadBatches(): void {
  for (const batch of batches.values()) {
    batch.discarded = true
    for (const item of batch.state.items) {
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl)
    }
    batch.files.clear()
    batch.converted.clear()
    batch.awaitingConfirm.clear()
    // An editor still mounted for a render or two must not keep showing the dropped items.
    setItems(batch, [], 0)
  }
  batches.clear()
  for (const url of handedOff) URL.revokeObjectURL(url)
  handedOff.clear()
}
