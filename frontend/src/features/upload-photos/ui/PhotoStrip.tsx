import { useCallback, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { type PostImage, Thumbnail } from '@/entities/image'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, Notice, Typography } from '@/shared/ui'
import type { UploadItem } from '../model/upload-batch'

interface PhotoStripProps {
  /** The post's photos — the server's, plus the ones this client has confirmed. */
  images: readonly PostImage[]
  /** This session's uploads for the post that are not photos yet. */
  items: readonly UploadItem[]
  onDelete: (image: PostImage) => void
  deletingId?: string
  deleteFailedId?: string
  deleteFailure?: AppFailure
  onRetry: (id: string) => void
  onDismiss: (id: string) => void
}

/** Saved photos first, then the ones still on their way. */
export function PhotoStrip({
  images,
  items,
  onDelete,
  deletingId,
  deleteFailedId,
  deleteFailure,
  onRetry,
  onDismiss,
}: PhotoStripProps) {
  const { t } = useTranslation(['posts', 'common'])
  // The photo the confirm sheet is asking about, and whether this sheet has already fired its
  // delete once. `deleteFailedId` is derived from the mutation and outlives the sheet, so
  // without the second flag reopening the sheet would greet the user with the last attempt's
  // error before they have pressed anything.
  const [confirmingId, setConfirmingId] = useState<string>()
  const [attempted, setAttempted] = useState(false)
  // Stable, because `Dialog` re-runs its focus effect whenever this identity changes and the
  // strip re-renders on every tick of a batch that is still uploading behind the sheet.
  const closeConfirm = useCallback(() => setConfirmingId(undefined), [])

  const inFlight = items.filter((item) => item.status !== 'skipped')
  const failed = inFlight.filter((item) => item.status === 'failed')
  const retryable = failed.filter((item) => item.failure === 'network')
  // The sheet closes itself when the photo leaves the post: a delete that lands drops it from
  // the cache, and there is nothing left to ask about.
  const confirming = images.find((image) => image.id === confirmingId)
  const deleting = confirming !== undefined && deletingId === confirming.id
  const deleteFailed =
    attempted && confirming !== undefined && deleteFailedId === confirming.id && !deleting

  // An empty section reads as a bug on a phone, where there is no other chrome around it
  // (design-language §7): the slot says what photos are for instead of collapsing. The body
  // role, not the 12px of a status line — it is a sentence the user is meant to act on (§3).
  if (images.length === 0 && inFlight.length === 0) {
    return (
      <Typography variant="body" className="text-content-tertiary">
        {t('upload.empty', { ns: 'posts' })}
      </Typography>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      {failed.length > 0 && (
        // The page-level signal for a lost upload. The per-tile message is 400px off the right
        // edge of a 360px screen behind a horizontal scroll nobody performs, so without this the
        // batch just quietly ends up one photo short (§4.3).
        <Notice tone="danger" role="alert">
          <span className="min-w-0">
            {t('upload.failedCount', { ns: 'posts', count: failed.length })}
          </span>
          {retryable.length > 0 && (
            // One tap for the whole batch: when the network drops mid-upload every failure is the
            // same failure, and retrying them one tile at a time is the wrong interaction.
            <Button
              variant="ghost"
              onClick={() => {
                for (const item of retryable) onRetry(item.id)
              }}
              className="text-notice-danger-fg shrink-0 underline"
            >
              {t('action.retry', { ns: 'common' })}
            </Button>
          )}
        </Notice>
      )}
      {/* The strip runs to both screen edges on a phone (§4) and snaps, so a flick parks a whole
          tile at the gutter instead of slicing the third one at the page's `px-4` — a clipped
          tile reads as a layout bug, not as "there is more". `overscroll-x-contain` keeps a flick
          that runs out of strip from chaining to the page (§8.2). */}
      <ul
        className="-mx-4 flex snap-x snap-mandatory scroll-px-4 gap-2 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:scroll-px-0 sm:px-0"
        aria-label={t('upload.photosAria', { ns: 'posts' })}
      >
        {images.map((image) => (
          <li key={image.id} className="snap-start">
            <Thumbnail
              src={image.viewUrl}
              alt={image.filename}
              width={image.width}
              height={image.height}
              dimmed={deletingId === image.id}
            >
              <Button
                variant="danger"
                size="icon"
                onClick={() => {
                  setConfirmingId(image.id)
                  setAttempted(false)
                }}
                disabled={deletingId === image.id}
                aria-label={t('upload.deleteAria', { ns: 'posts', filename: image.filename })}
                className="bg-media-scrim-bg hover:bg-media-scrim-bg active:bg-media-scrim-bg absolute top-1 right-1"
              >
                <X aria-hidden="true" className="size-5" />
              </Button>
            </Thumbnail>
          </li>
        ))}
        {inFlight.map((item) => (
          <li key={item.id} className="snap-start">
            <Thumbnail src={item.previewUrl} alt={item.filename} dimmed>
              {item.status === 'failed' ? (
                <Overlay>
                  {item.appFailure ? (
                    <Typography variant="body" as="div">
                      <AppFailureMessage failure={item.appFailure} />
                    </Typography>
                  ) : (
                    <Typography variant="body" as="span">
                      {item.failure &&
                        t(`upload.failure.${failureKey(item.failure)}`, { ns: 'posts' })}
                    </Typography>
                  )}
                  {/* One full-width action per tile. Two 44px targets side by side inside a 128px
                      square shrink to ~45px and break their Korean labels mid-word, and stacked
                      they do not fit under the reason at all (§4.1) — so the tile keeps the
                      action that is always valid and the retry lives in the notice above. */}
                  <Button
                    variant="ghost"
                    onClick={() => onDismiss(item.id)}
                    className="text-media-scrim-fg hover:bg-media-scrim-bg active:bg-media-scrim-bg w-full underline"
                  >
                    {t('upload.dismiss', { ns: 'posts' })}
                  </Button>
                </Overlay>
              ) : (
                <Overlay>
                  <Typography variant="body" as="span">
                    {t(`upload.status.${statusKey(item.status)}`, { ns: 'posts' })}
                  </Typography>
                </Overlay>
              )}
            </Thumbnail>
          </li>
        ))}
      </ul>
      {confirming && (
        // Deleting a photo takes the object with it (spec/policy/uploads.md) and the converted
        // copy is already gone, so there is nothing to undo — §7 confirms exactly this through
        // the sheet. It also takes the failure out of the tile: a scrim on a 128px square has no
        // room for a way out, and the sheet already has 취소 beside the retry.
        <Dialog
          open
          title={t('upload.deleteTitle', { ns: 'posts' })}
          confirmLabel={t('action.delete', { ns: 'common' })}
          pending={deleting}
          onConfirm={() => {
            setAttempted(true)
            onDelete(confirming)
          }}
          onClose={closeConfirm}
        >
          {/* The filename comes from the server, so it breaks inside the sheet rather than
              widening it (§3.2). */}
          {t('upload.deleteDescription', { ns: 'posts', filename: confirming.filename })}
          {deleteFailed && (
            <Notice tone="danger" role="alert" className="mt-3">
              {deleteFailure ? (
                <div className="grid gap-1">
                  <span>{t('upload.deleteFailed', { ns: 'posts' })}</span>
                  <AppFailureMessage failure={deleteFailure} />
                </div>
              ) : (
                t('upload.deleteFailed', { ns: 'posts' })
              )}
            </Notice>
          )}
        </Dialog>
      )}
    </div>
  )
}

function failureKey(failure: 'network' | 'duplicate-filename' | 'invalid') {
  return failure === 'duplicate-filename' ? 'duplicateFilename' : failure
}

function statusKey(status: UploadItem['status']) {
  return status === 'selected' ? 'selected' : status === 'converting' ? 'converting' : 'uploading'
}

function Overlay({ children }: { children: ReactNode }) {
  return (
    <div className="bg-media-scrim-bg/90 text-media-scrim-fg absolute inset-0 flex flex-col items-center justify-center gap-2 p-2 text-center">
      {children}
    </div>
  )
}
