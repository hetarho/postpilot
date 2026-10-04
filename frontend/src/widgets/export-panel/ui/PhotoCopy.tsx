import { useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import type { PostImage } from '@/entities/image'
import type { PhotoFit } from '@/entities/post'
import { presignExpired } from '@/shared/lib'
import { RotatedImage, Typography } from '@/shared/ui'
import type { FailedCopyKind } from '../model/copy-target'

export interface PhotoCopyProps {
  file: string
  alt: string
  image: PostImage | undefined
  /** This photo's number, from 0. The number shown is `marker + 1`, matching the numbers in the
   *  copied text, which count every photo from 1, alone or in a group (EXPORT-5). */
  marker: number
  copied: boolean
  failure: FailedCopyKind | undefined
  /** The photo's turn rides along so the copy is turned as the screen shows it (EXPORT-15). */
  onCopy: (element: HTMLImageElement, rotation: number) => void
  onStale: (() => void) | undefined
  /** Set inside a photo group: the cell shape the group's layout gives the photo (POST-105). */
  fit?: PhotoFit
}

/** One photo as its own copy control (EXPORT-12), with its own status line under it — alone, or
 *  as one cell of a photo group. */
export function PhotoCopy({
  file,
  alt,
  image,
  marker,
  copied,
  failure,
  onCopy,
  onStale,
  fit,
}: PhotoCopyProps) {
  const { t } = useTranslation('posts')
  const statusId = useId()
  // A just-confirmed upload can still be carrying its local blob preview in the post cache. Those
  // bytes are not the stored photo, so the copy is not offered for them — the same rule the
  // contact sheet applies to a server-read surface. The pixels still render: they are the photo
  // the reader will see.
  const url = image && !image.viewUrl.startsWith('blob:') ? image.viewUrl : ''
  const imageRef = useRef<HTMLImageElement>(null)
  // A presigned view URL expires. Keyed BY URL rather than as a bare boolean, so a refresh that
  // remints it clears the failure without any reset plumbing. The KIND rides along because a
  // photo that never painted and a photo this origin may not read are the same event here and
  // opposite advice — see `classifyLoadFailure`.
  const [loadFailure, setLoadFailure] = useState<{ url: string; kind: FailedCopyKind }>()
  // One refresh per photo per mount, counted rather than keyed by url: every refresh mints a NEW
  // url, so a per-url guard would let a bucket that refuses this origin drive an unbounded
  // refetch loop — fail, remint, fail, remint. A ref, not state; nothing renders from it.
  const refreshesAsked = useRef(0)
  const unreachable = url === '' || loadFailure?.url === url
  // Keyed by kind rather than chained, so a kind added to `CopyImageResult` is a type error here
  // instead of a photo that fails silently — which is how `blocked` and `unreadable` came to share
  // one message and send users to reload a post over a rule that reloading cannot change.
  const failureMessage: Record<FailedCopyKind, string> = {
    unsupported: t('export.photoUnsupported'),
    refused: t('export.photoRefused'),
    blocked: t('export.photoBlocked'),
    unreadable: t('export.photoUnreadable'),
  }
  const reason = !image
    ? t('export.photoMissing')
    : url === ''
      ? // A `blob:` preview: recovered by the reload that replaces it with the stored photo, and
        // it offers no copy to fail in the first place.
        t('export.photoUnreadable')
      : loadFailure?.url === url
        ? failureMessage[loadFailure.kind]
        : failure
          ? failureMessage[failure]
          : ''

  /** A photo that did not paint, split into the two things it can mean.
   *
   *  The element is CORS-loaded, so this fires for a URL whose lifetime ran out AND for a bucket
   *  that allows this origin no `GET` — R2 answers both without CORS headers and the browser
   *  reports neither. The URL's own lifetime is what separates them (`presignExpired`), and they
   *  lead to opposite advice: the first is refreshed away below, the second cannot be, so it says
   *  to place the photo by hand instead of starting a reload loop with no exit. */
  function classifyLoadFailure(failed: string) {
    setLoadFailure({
      url: failed,
      kind: presignExpired(failed, Date.now()) ? 'unreadable' : 'blocked',
    })
    // Asked for even on the `blocked` reading: a transient network fault looks exactly like it
    // from here, and one refetch is what tells them apart — a fresh URL that paints was never a
    // bucket rule. Concurrent asks from the other photos collapse into one refetch, and the
    // message above clears by itself the moment a url that paints replaces this one.
    if (refreshesAsked.current === 0) {
      refreshesAsked.current = 1
      onStale?.()
    }
  }

  return (
    <div>
      {image ? (
        image.viewUrl ? (
          /* The PHOTO is the control (no overlaid button): a 44px target in the corner of a photo
             that fills the column was a quarter of the reach it needed, and the corner was also
             where the thumb rests while scrolling. The `<img>` stays a real `<img>` INSIDE the
             button rather than under an invisible overlay, so the browser's own 이미지 복사 stays
             on the right-click menu — the workaround that carried this before the copy read
             pixels. Focus takes the app-wide `:focus-visible` outline; the press treatment is on
             the photo itself because a fill behind it would never be seen. */
          <button
            type="button"
            disabled={unreachable}
            aria-label={t('export.photoCopyAria', { number: marker + 1, file })}
            aria-describedby={reason ? statusId : undefined}
            onClick={() => imageRef.current && onCopy(imageRef.current, image.rotation ?? 0)}
            className="block w-full cursor-pointer rounded-lg active:brightness-90 disabled:cursor-default disabled:active:brightness-100"
          >
            <RotatedImage
              fit={fit ? 'fill' : 'natural'}
              rotation={image.rotation}
              frameClassName="rounded-lg"
              imgRef={imageRef}
              src={image.viewUrl}
              alt={alt || file}
              width={image.width}
              height={image.height}
              // NOT lazy, and CORS-loaded. The copy reads the pixels this element already holds,
              // so a photo has to have PAINTED to be copyable — deferring the load until it is
              // scrolled to would defer it past the URL's lifetime on exactly the panel that is
              // left open. `crossOrigin` is what keeps the canvas origin-clean; without it the
              // encode is refused for a photo that is plainly on screen (DEPLOY.md §5).
              crossOrigin="anonymous"
              decoding="async"
              onError={() => classifyLoadFailure(url || image.viewUrl)}
              className={
                fit === 'cover'
                  ? 'bg-surface-recessed aspect-square w-full rounded-lg object-cover'
                  : fit === 'contain'
                    ? 'bg-surface-recessed aspect-square w-full rounded-lg object-contain'
                    : 'bg-surface-recessed h-auto w-full rounded-lg'
              }
            />
          </button>
        ) : (
          // The view URL is minted per GetPost; until one arrives the box is still held open so
          // the text below it does not jump when the photo paints.
          <div className="bg-surface-recessed aspect-square w-full rounded-lg" />
        )
      ) : (
        // No photo behind this marker: the copied text still spends its number on it, so the
        // preview says which file the reader is expected to place there — the marker itself
        // carries no filename any more (EXPORT-5).
        <Typography
          variant="meta"
          as="p"
          className={twMerge(
            'bg-surface-recessed rounded-lg px-4 py-3 break-words',
            fit && 'flex aspect-square items-center break-all',
          )}
        >
          {file}
        </Typography>
      )}
      {/* Always mounted: a live region inserted with its text already inside announces nothing.
          It is also the disabled control's reason, which is why the control points at it — a
          disabled button is skipped by the keyboard and would otherwise carry no explanation. */}
      <Typography
        variant="meta"
        as="p"
        id={statusId}
        role="status"
        className="mt-1 min-h-4 break-words"
      >
        {copied ? t('export.photoCopied', { file }) : reason}
      </Typography>
    </div>
  )
}
