import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Copy } from 'lucide-react'
import { type PostImage } from '@/entities/image'
import { BlockList, imageByFile } from '@/entities/post'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { toSite } from '@/features/export-site'
import { toTistory } from '@/features/export-tistory'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
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
  // separately and the confirmation lands on the entry that was pressed. A TEXT copy stores the
  // value that reached the clipboard beside it: the Naver tab's copy carries no fallback element
  // whose value `isCurrent` could compare, so without this a copy racing a content change could
  // announce 복사됨 for a body the post no longer contains — the value comparison in the status
  // derivations below is what drops that stale confirmation.
  const [copied, setCopied] = useState<{ target: CopyTarget; value?: string }>()
  // Which control's copy fell back to manual selection, so its hint renders beside that control
  // rather than somewhere the user is not looking (§4.3). On the Naver tab this also REVEALS the
  // raw marker text: its default view is the rendered post, and a selection needs a text field.
  // The VALUE that fell back is stored with it, so the fallback dissolves by derivation the moment
  // the content no longer matches what was selected — no effect resetting state over a prop. The
  // CONTENT IDENTITY rides along because the value alone would resurrect a dismissed fallback when
  // an edit is undone (the output string comes back; the failed copy does not).
  const [manualCopy, setManualCopy] = useState<{
    target: 'output' | 'title'
    value: string
    source: PostContent
  }>()
  // Per-photo failure kind, keyed the same way. It is separate from `manualCopy` because a
  // photo has no manual fallback at all — there is nothing to select and hold (see `copyImage`).
  const [photoFailure, setPhotoFailure] = useState<{ target: CopyTarget; kind: FailedCopyKind }>()
  const outputRef = useRef<HTMLTextAreaElement>(null)
  const titleRef = useRef<HTMLInputElement>(null)
  const copyButtonRef = useRef<HTMLButtonElement>(null)
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
  // Marker index per block index, from the SAME canonical block array `toNaver` walks, so a photo
  // in the preview and a `[사진 …]` marker in the copied text cannot drift apart: they match by
  // position. The marker index — not the block index — is the copy target's identity, unchanged
  // from the strip this preview replaces.
  const markerIndexByBlock = useMemo(() => {
    const map = new Map<number, number>()
    content.blocks.forEach((block, index) => {
      if (block.type === BlockType.IMAGE) map.set(index, map.size)
    })
    return map
  }, [content])
  const hasPhotos = markerIndexByBlock.size > 0
  const imagesByFilename = useMemo(() => imageByFile(images), [images])
  const formatOptions = EXPORT_FORMATS.map((value) => ({
    value,
    label: t(`export.formatLabel.${value}`),
  }))
  // The Naver tab shows the rendered post; the raw marker text exists only on the clipboard —
  // except while a refused copy needs a visible selection to fall back to. The other three
  // formats are markup meant to be read as source, so they keep the raw field always. The value
  // comparison is what dismisses the fallback when the content changes under it.
  const outputFellBack =
    manualCopy?.target === 'output' && manualCopy.value === output && manualCopy.source === content
  const rawFieldVisible = format !== 'naver' || outputFellBack

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
    setCopied(undefined)
    setManualCopy(undefined)
    setPhotoFailure(undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
  }

  // The manual fallback must be SEEN to be used: on the Naver tab the raw field mounts only after
  // the copy has fallen back, so `copyText` was handed no element and the selection happens here,
  // once the field exists. The dismissal side is the same effect's business: a content change
  // dissolves the fallback by derivation and UNMOUNTS the focused field, which would drop the
  // keyboard onto <body> — the focus is handed back to the copy button instead. A dismissal by
  // tab switch or by pressing another control leaves focus where the user put it.
  const fallbackWasRevealed = useRef(false)
  useEffect(() => {
    if (format === 'naver' && outputFellBack) {
      fallbackWasRevealed.current = true
      outputRef.current?.focus()
      outputRef.current?.select()
      return
    }
    if (fallbackWasRevealed.current) {
      fallbackWasRevealed.current = false
      if (document.activeElement === document.body) copyButtonRef.current?.focus()
    }
  }, [format, outputFellBack])

  /** The image copy, on the SAME discipline as the text copy above: one generation counter so a
   *  stale async result cannot land, one queue so two presses do not race for the clipboard, the
   *  same `COPY_FEEDBACK_MS` dwell, and the same always-mounted live region. It reports the
   *  failure KIND instead of a manual-selection hint, because an image has no manual fallback. */
  async function copyPhoto(target: CopyTarget, url: string) {
    const generation = ++copyGeneration.current
    const isCurrent = () => mounted.current && copyGeneration.current === generation
    setCopied(undefined)
    setManualCopy(undefined)
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
      setCopied({ target })
      feedbackTimer.current = window.setTimeout(() => setCopied(undefined), COPY_FEEDBACK_MS)
      return
    }
    setPhotoFailure({ target, kind: result.kind })
  }

  /** `fallback` is null when the manual field is not mounted yet (the Naver preview): the copy is
   *  still attempted, and a refusal reveals the field — the effect above then selects it. */
  async function copy(
    target: 'output' | 'title',
    value: string,
    fallback: CopyFallbackElement | null,
  ) {
    const generation = ++copyGeneration.current
    const isCurrent = () =>
      mounted.current &&
      copyGeneration.current === generation &&
      (fallback === null ||
        (fallback.value === value &&
          (target === 'output' ? outputRef.current === fallback : titleRef.current === fallback)))
    setCopied(undefined)
    // `manualCopy` is NOT cleared up front the way the other feedback is: on the Naver tab it is
    // what keeps the revealed fallback field mounted, and clearing it here would unmount the field
    // the user is retrying from for the whole in-flight wait. The outcome below overwrites it.
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
    setManualCopy(result.copied ? undefined : { target, value, source: content })
    setCopied(result.copied ? { target, value } : undefined)
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current)
    if (result.copied) {
      feedbackTimer.current = window.setTimeout(() => {
        setCopied(undefined)
      }, COPY_FEEDBACK_MS)
    }
  }

  // One line per copy target, mounted whether or not it has anything to say: a live region has to
  // exist BEFORE its text changes or a screen reader announces nothing, and the sole confirmation
  // used to be a 1.5s label swap on the button under the thumb that hid it.
  const titleStatus =
    copied?.target === 'title' && copied.value === content.title
      ? t('export.titleCopied')
      : manualCopy?.target === 'title' &&
          manualCopy.value === content.title &&
          manualCopy.source === content
        ? t('export.manualCopy')
        : ''
  const outputStatus =
    copied?.target === 'output' && copied.value === output
      ? t('action.copied', { ns: 'common' })
      : outputFellBack
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
            none would otherwise be told to copy photos from a preview that renders none. */}
        <Typography variant="label" as="p">
          {format === 'naver' && hasPhotos
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

        {/* The copy action sits ABOVE the output, not after it. This panel renders inside the
            editor, which already docks its own bar, and two docked bars in one scroller stick to
            the same offset and paint over each other — so §4.3's other option applies: put the
            action where the user already is. Above the field it shares a screen with the format
            tabs and the first lines of the result, which is what the user checks before copying;
            after an autoGrow field it would be a whole post's length away. */}
        <div className="mt-4 flex flex-col gap-2 sm:flex-row-reverse sm:items-center sm:justify-end sm:gap-3">
          {/* The label stays 복사: a swap to 복사됨 resizes the target under the thumb that just
              pressed it, and it was also the only signal that anything had happened (§6). */}
          {/* `outputRef.current` is naturally null while the Naver tab shows the preview and the
              live field on every other state — including a Naver retry from the revealed field. */}
          <Button
            ref={copyButtonRef}
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

        {rawFieldVisible ? (
          <>
            <FieldLabel htmlFor="export-output" className="sr-only">
              {t('export.result')}
            </FieldLabel>
            {/* A Korean post runs to ~58 lines at this width. A fixed 18-row box scrolled
                internally, so every vertical swipe that landed on it moved the output instead of
                the page and the only place left to scroll was the 16px gutter (§4.4) — it grows
                instead. No mono face: Tailwind's stock mono stack carries no Hangul, so every
                glyph fell back (§3). */}
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
          </>
        ) : (
          /* The Naver preview IS the post (change 18): what SmartEditor gets is plain text with
             `[사진 …]` markers, but what the human reads here is the rendering they are about to
             publish — photos inline at their marker positions, each carrying its own copy. The
             header is suppressed because the body copy does not paste it; the title has its own
             field above. */
          <BlockList
            content={content}
            images={images}
            label={t('export.preview')}
            className="mt-3 pb-0"
            renderHeader={() => null}
            renderBlock={(block, index, rendered) => {
              if (block.type !== BlockType.IMAGE) return rendered
              const target: CopyTarget = `photo:${markerIndexByBlock.get(index) ?? 0}:${block.file}`
              return (
                <PreviewPhoto
                  file={block.file}
                  alt={block.alt}
                  caption={block.caption}
                  image={imagesByFilename.get(block.file)}
                  copied={copied?.target === target}
                  failure={photoFailure?.target === target ? photoFailure.kind : undefined}
                  onCopy={(url) => void copyPhoto(target, url)}
                />
              )
            }}
            // A marker whose photo is missing keeps its POSITION: dropping it would shift every
            // later photo against its marker — the one thing marker order exists to prevent.
            renderMissingImage={(block) => (
              <PreviewPhoto
                file={block.file}
                alt=""
                caption={block.caption}
                image={undefined}
                copied={false}
                failure={undefined}
                onCopy={() => undefined}
              />
            )}
          />
        )}
      </div>
    </section>
  )
}

/** One inline photo of the Naver preview: the pixels at their natural width, the copy control
 *  overlaid at the top-right, and that photo's own status line.
 *
 *  It reports its own state under its own photo rather than in one shared line, because "which
 *  photo failed" is the only useful part of the message (§4.3). */
function PreviewPhoto({
  file,
  alt,
  caption,
  image,
  copied,
  failure,
  onCopy,
}: {
  file: string
  alt: string
  caption: string
  image: PostImage | undefined
  copied: boolean
  failure: FailedCopyKind | undefined
  onCopy: (url: string) => void
}) {
  const { t } = useTranslation('posts')
  const statusId = useId()
  // A just-confirmed upload can still be carrying its local blob preview in the post cache. Those
  // bytes are not the stored photo, so the copy is not offered for them — the same rule the
  // contact sheet applies to a server-read surface. The pixels still render: they are the photo
  // the reader will see.
  const url = image && !image.viewUrl.startsWith('blob:') ? image.viewUrl : ''
  // A presigned view URL expires. Keyed BY URL rather than as a bare boolean, so a reload that
  // remints it clears the failure without any reset plumbing.
  const [failedUrl, setFailedUrl] = useState('')
  const unreachable = url === '' || failedUrl === url
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
    : // An `<img>` that never painted, or a `blob:` preview: both are recovered by the reload that
      // remints the URL, and neither offers a copy control to fail in the first place.
      unreachable
      ? t('export.photoUnreadable')
      : failure
        ? failureMessage[failure]
        : ''
  return (
    <div className="py-2">
      {image ? (
        <div className="relative">
          {image.viewUrl ? (
            <img
              src={image.viewUrl}
              alt={alt || file}
              width={image.width}
              height={image.height}
              loading="lazy"
              decoding="async"
              onError={() => setFailedUrl(url)}
              className="bg-surface-recessed h-auto w-full rounded-lg"
            />
          ) : (
            // The view URL is minted per GetPost; until one arrives the box is still held open so
            // the text below it does not jump when the photo paints.
            <div className="bg-surface-recessed aspect-square w-full rounded-lg" />
          )}
          {/* Over the photo, at its top-right — job 44 kept the control BESIDE the strip's 128px
              tile because 44px covered a quarter of it; an inline photo fills the column, so the
              same control covers a corner. The scrim is its own plane so it stays legible against
              both a bright and a dark photo (§4.1), on every state: `hover:`/`active:` would
              otherwise swap in the secondary fill and drop the white glyph on it. */}
          <Button
            variant="secondary"
            size="icon"
            disabled={unreachable}
            aria-label={t('export.photoCopyAria', { file })}
            aria-describedby={reason ? statusId : undefined}
            onClick={() => onCopy(url)}
            className="bg-media-scrim-bg text-media-scrim-fg hover:bg-media-scrim-bg active:bg-media-scrim-bg absolute top-2 right-2"
          >
            <Copy aria-hidden="true" className="size-5" />
          </Button>
        </div>
      ) : (
        // No photo behind this marker: the copied text still carries `[사진 <file>]`, so the
        // preview says which file the reader is expected to place there.
        <Typography
          variant="meta"
          as="p"
          className="bg-surface-recessed rounded-lg px-4 py-3 break-words"
        >
          {file}
        </Typography>
      )}
      {caption && (
        <Typography variant="label" as="p" className="mt-2 break-words">
          {caption}
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
