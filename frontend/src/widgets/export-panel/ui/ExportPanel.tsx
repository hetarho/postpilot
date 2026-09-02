import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Thumbnail, type PostImage } from '@/entities/image'
import { toMarkdown } from '@/features/export-markdown'
import { naverPhotoOrder, toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import type { ContentLanguage, PostContent } from '@/shared/api'
import { COPY_FEEDBACK_MS } from '@/shared/config'
import { copyImage, copyText, type CopyFallbackElement, type CopyImageResult } from '@/shared/lib'
import { Button, FieldLabel, SegmentedControl, Textarea, TextField, Typography } from '@/shared/ui'
import { EXPORT_FORMATS, type ExportFormat } from '../config/guidance'

/** `output` and `title` are the two text copies; a photo target names the marker it belongs to,
 *  so one file carrying two markers is still two independent copies. It is a template literal
 *  rather than a bare `string`, which would collapse the union and take the checking with it. */
type CopyTarget = 'output' | 'title' | `photo:${number}:${string}`

/** Every way a photo copy can fail, from `copyImage`. `copied` is not one of them. */
type FailedCopyKind = Exclude<CopyImageResult['kind'], 'copied'>

interface ExportPanelProps {
  content: PostContent
  images: readonly PostImage[]
  createdAt: string
  contentLanguage: ContentLanguage
}

/** Four synchronous browser-only derivations of the canonical block array. */
export function ExportPanel({ content, images, createdAt, contentLanguage }: ExportPanelProps) {
  const { t } = useTranslation('posts')
  const [format, setFormat] = useState<ExportFormat>('naver')
  // A photo target names the marker it belongs to, so two markers for one file still report
  // separately and the confirmation lands on the entry that was pressed.
  const [copiedTarget, setCopiedTarget] = useState<CopyTarget>()
  // Which control's copy fell back to manual selection, so its hint renders beside that control
  // rather than somewhere the user is not looking (§4.3).
  const [manualCopyTarget, setManualCopyTarget] = useState<CopyTarget>()
  // Per-photo failure kind, keyed the same way. It is separate from `manualCopyTarget` because a
  // photo has no manual fallback at all — there is nothing to select and hold (see `copyImage`).
  const [photoFailure, setPhotoFailure] = useState<{ target: CopyTarget; kind: FailedCopyKind }>()
  const outputRef = useRef<HTMLTextAreaElement>(null)
  const titleRef = useRef<HTMLInputElement>(null)
  const feedbackTimer = useRef<number | undefined>(undefined)
  const copyGeneration = useRef(0)
  const copyQueue = useRef<Promise<void>>(Promise.resolve())
  const mounted = useRef(false)
  const outputs = useMemo(
    () => ({
      naver: toNaver(content, images, contentLanguage),
      tistory: toTistory(content, images, contentLanguage),
      site: toSite(content, images, createdAt, contentLanguage),
      markdown: toMarkdown(content, images, createdAt, contentLanguage),
    }),
    [content, contentLanguage, createdAt, images],
  )
  const output = outputs[format]
  // The strip's order is the MARKER order, derived from the same block array the text is, so a
  // photo on screen and a marker in the pasted text cannot drift apart.
  const photoEntries = useMemo(() => {
    if (format !== 'naver') return []
    const byFilename = new Map(images.map((image) => [image.filename, image]))
    return naverPhotoOrder(content).map((file, index) => ({
      // The marker's POSITION is part of the identity: one file can carry two markers, and each
      // one is its own copy control with its own confirmation.
      target: `photo:${index}:${file}` as const,
      file,
      image: byFilename.get(file),
    }))
  }, [content, format, images])
  const formatOptions = EXPORT_FORMATS.map((value) => ({
    value,
    label: t(`export.formatLabel.${value}`),
  }))

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      copyGeneration.current += 1
      if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    }
  }, [])

  function invalidateCopyFeedback() {
    copyGeneration.current += 1
    setCopiedTarget(undefined)
    setManualCopyTarget(undefined)
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
  }

  /** The image copy, on the SAME discipline as the text copy above: one generation counter so a
   *  stale async result cannot land, one queue so two presses do not race for the clipboard, the
   *  same `COPY_FEEDBACK_MS` dwell, and the same always-mounted live region. It reports the
   *  failure KIND instead of a manual-selection hint, because an image has no manual fallback. */
  async function copyPhoto(target: CopyTarget, url: string) {
    const generation = ++copyGeneration.current
    const isCurrent = () => mounted.current && copyGeneration.current === generation
    setCopiedTarget(undefined)
    setManualCopyTarget(undefined)
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)

    // The result is READ OUT of the chained promise rather than written into a mutable outer
    // variable the way the text copy does: an image copy answers with a kind, and a `let` holding
    // one narrows to the literal it was initialized with.
    const operation = copyQueue.current
      .then(() => copyImage(url))
      .catch((): CopyImageResult => ({ kind: 'unreadable' }))
    copyQueue.current = operation.then(() => undefined)
    const result = await operation
    if (!isCurrent()) return
    if (result.kind === 'copied') {
      setCopiedTarget(target)
      feedbackTimer.current = window.setTimeout(() => setCopiedTarget(undefined), COPY_FEEDBACK_MS)
      return
    }
    setPhotoFailure({ target, kind: result.kind })
  }

  async function copy(
    target: 'output' | 'title',
    value: string,
    fallback: CopyFallbackElement | null,
  ) {
    const generation = ++copyGeneration.current
    const isCurrent = () =>
      mounted.current &&
      copyGeneration.current === generation &&
      fallback !== null &&
      fallback.value === value &&
      (target === 'output' ? outputRef.current === fallback : titleRef.current === fallback)
    setCopiedTarget(undefined)
    setManualCopyTarget(undefined)
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)

    let result = { copied: false }
    const operation = copyQueue.current
      .then(async () => {
        result = await copyText(value, fallback, isCurrent)
      })
      .catch(() => {
        result = { copied: false }
      })
    copyQueue.current = operation
    await operation
    if (!isCurrent()) return
    setManualCopyTarget(result.copied ? undefined : target)
    setCopiedTarget(result.copied ? target : undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    if (result.copied) {
      feedbackTimer.current = window.setTimeout(() => {
        setCopiedTarget(undefined)
      }, COPY_FEEDBACK_MS)
    }
  }

  // One line per copy target, mounted whether or not it has anything to say: a live region has to
  // exist BEFORE its text changes or a screen reader announces nothing, and the sole confirmation
  // used to be a 1.5s label swap on the button under the thumb that hid it.
  const titleStatus =
    copiedTarget === 'title'
      ? t('export.titleCopied')
      : manualCopyTarget === 'title'
        ? t('export.manualCopy')
        : ''
  const outputStatus =
    copiedTarget === 'output'
      ? t('action.copied', { ns: 'common' })
      : manualCopyTarget === 'output'
        ? t('export.manualCopy')
        : ''

  return (
    <section aria-labelledby="export-heading" className="mt-10">
      <Typography variant="title" id="export-heading">
        {t('export.title')}
      </Typography>
      {/* The four Korean format names measure ~380px in one row against 328px of content at 360px,
          which cut 마크다운 in half with no scrollbar to say so. Two columns at the base
          breakpoint fit all four; the strip comes back where the width exists. */}
      <SegmentedControl
        value={format}
        options={formatOptions}
        ariaLabel={t('export.format')}
        controls="export-output-panel"
        onChange={(next) => {
          invalidateCopyFeedback()
          setFormat(next)
        }}
        className="mt-4 grid grid-cols-2 sm:flex"
      />

      <div id="export-output-panel" role="tabpanel" className="mt-4">
        {/* The Naver guidance describes the photo flow only when there ARE photos: a post with
            none would otherwise be told to copy each photo from a strip that is not rendered. */}
        <Typography variant="label" as="p">
          {format === 'naver' && photoEntries.length > 0
            ? t('export.guidance.naverPhotos')
            : t(`export.guidance.${format}`)}
        </Typography>

        {format === 'naver' && (
          <div className="mt-4">
            <FieldLabel htmlFor="export-title">{t('export.naverTitle')}</FieldLabel>
            <div className="mt-2 flex items-center gap-2">
              {/* `min-w-0` on the field, `shrink-0` on the button: the title is a server string
                  and must be the thing that gives way, never the control beside it (§8.5). */}
              <TextField
                id="export-title"
                ref={titleRef}
                value={content.title}
                readOnly
                className="min-w-0 flex-1"
              />
              <Button
                variant="secondary"
                className="shrink-0"
                onClick={() => void copy('title', content.title, titleRef.current)}
              >
                {t('export.copyTitle')}
              </Button>
            </div>
            <Typography
              variant="body"
              as="p"
              role="status"
              className="text-content-tertiary mt-1 min-h-5"
            >
              {titleStatus}
            </Typography>
          </div>
        )}

        <FieldLabel htmlFor="export-output" className="sr-only">
          {t('export.result')}
        </FieldLabel>
        {/* The copy action sits ABOVE the output, not after it. This panel renders inside the
            editor, which already docks its own bar, and two docked bars in one scroller stick to
            the same offset and paint over each other — so §4.3's other option applies: put the
            action where the user already is. Above the field it shares a screen with the format
            tabs and the first lines of the result, which is what the user checks before copying;
            after an autoGrow field it would be a whole post's length away. */}
        <div className="mt-4 flex flex-col gap-2 sm:flex-row-reverse sm:items-center sm:justify-end sm:gap-3">
          {/* The label stays 복사: a swap to 복사됨 resizes the target under the thumb that just
              pressed it, and it was also the only signal that anything had happened (§6). */}
          <Button
            variant="cta"
            className="w-full sm:w-auto"
            onClick={() => void copy('output', output, outputRef.current)}
          >
            {t('action.copy', { ns: 'common' })}
          </Button>
          {/* Always mounted, never conditionally inserted: a live region that first appears WITH
              its text already in it is not announced (§9). */}
          <Typography variant="body" as="p" role="status" className="text-content-tertiary min-h-5">
            {outputStatus}
          </Typography>
        </div>
        {/* A Korean post runs to ~58 lines at this width. A fixed 18-row box scrolled internally,
            so every vertical swipe that landed on it moved the output instead of the page and the
            only place left to scroll was the 16px gutter (§4.4) — it grows instead. No mono face:
            Tailwind's stock mono stack carries no Hangul, so every glyph fell back (§3). */}
        <Textarea
          id="export-output"
          ref={outputRef}
          value={output}
          readOnly
          spellCheck={false}
          rows={8}
          autoGrow
          className="mt-3"
        />

        {/* The photo strip, for Naver alone. The text carries `[사진 …]` markers that SmartEditor
            ONE cannot resolve on its own, so each photo is copied and pasted at its marker; the
            other three formats resolve their own images and get no strip. */}
        {photoEntries.length > 0 && (
          <section aria-labelledby="export-photos-heading" className="mt-8">
            <Typography variant="fieldTitle" id="export-photos-heading">
              {t('export.photos')}
            </Typography>
            <Typography variant="label" as="p" className="mt-1">
              {t('export.photosHelp')}
            </Typography>
            {/* One horizontal snap carousel, like every other photo strip in the app (§4.4,
                §8.4): the entries are square tiles and a 360px screen holds about two and a
                half of them. */}
            <ul className="-mx-4 mt-3 flex snap-x snap-mandatory scroll-px-4 gap-3 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:scroll-px-0 sm:px-0">
              {photoEntries.map((entry) => (
                <li key={entry.target} className="w-32 shrink-0 snap-start">
                  <PhotoEntry
                    file={entry.file}
                    image={entry.image}
                    copied={copiedTarget === entry.target}
                    failure={photoFailure?.target === entry.target ? photoFailure.kind : undefined}
                    onCopy={(url) => void copyPhoto(entry.target, url)}
                  />
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </section>
  )
}

/** One photo of the strip: the pixels, the marker filename that matches it to the pasted text,
 *  and its own copy control.
 *
 *  It reports its own state on its own tile rather than in one shared line, because "which photo
 *  failed" is the only useful part of the message (§4.3). */
function PhotoEntry({
  file,
  image,
  copied,
  failure,
  onCopy,
}: {
  file: string
  image: PostImage | undefined
  copied: boolean
  failure: FailedCopyKind | undefined
  onCopy: (url: string) => void
}) {
  const { t } = useTranslation('posts')
  const statusId = useId()
  // A just-confirmed upload can still be carrying its local blob preview in the post cache. Those
  // bytes are not the stored photo, so the copy is not offered for them — the same rule the
  // contact sheet applies to a server-read surface.
  const url = image && !image.viewUrl.startsWith('blob:') ? image.viewUrl : ''
  // A presigned view URL expires. Keyed BY URL rather than as a bare boolean, so a reload that
  // remints it clears the failure without any reset plumbing.
  const [failedUrl, setFailedUrl] = useState('')
  const unreachable = url === '' || failedUrl === url
  const reason = !image
    ? t('export.photoMissing')
    : unreachable
      ? t('export.photoUnreadable')
      : failure === 'unsupported'
        ? t('export.photoUnsupported')
        : failure === 'refused'
          ? t('export.photoRefused')
          : failure === 'unreadable'
            ? t('export.photoUnreadable')
            : ''
  return (
    <div className="grid gap-2">
      <Thumbnail
        src={url || undefined}
        alt={file}
        width={image?.width}
        height={image?.height}
        onError={() => setFailedUrl(url)}
      />
      {/* The marker filename, not a caption: it is what the user matches against the pasted text,
          so it wraps rather than truncating away the part that differs. A `figcaption` would have
          to be its figure's first or last child, and `Thumbnail` brings its own figure. */}
      <Typography variant="meta" as="p" className="break-words">
        {file}
      </Typography>
      {/* Beside the photo, not laid over it: a control over arbitrary photo pixels needs its own
          plane to stay legible against both a bright and a dark image, and at the 44px floor that
          plane covers a quarter of a 128px tile — which hides the thing the user is identifying.
          The `secondary` fill is the plane, and the tile above it stays unobscured (§4.1, §6,
          §8.4). Full size, not `compact`: §7 reserves that one sub-44px step for a low-emphasis
          way out sharing a dock, and this is the feature's own repeated action. */}
      <Button
        variant="secondary"
        className="w-full"
        aria-label={t('export.photoCopyAria', { file })}
        aria-describedby={reason ? statusId : undefined}
        disabled={unreachable}
        onClick={() => onCopy(url)}
      >
        {t('export.photoCopy')}
      </Button>
      {/* Always mounted: a live region inserted with its text already inside announces nothing.
          It is also the disabled control's reason, which is why the control points at it — a
          disabled button is skipped by the keyboard and would otherwise carry no explanation. */}
      <Typography variant="meta" as="p" id={statusId} role="status" className="min-h-4 break-words">
        {copied ? t('export.photoCopied', { file }) : reason}
      </Typography>
    </div>
  )
}
