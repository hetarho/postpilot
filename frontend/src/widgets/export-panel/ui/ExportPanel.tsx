import { useTranslation } from 'react-i18next'
import { type PostImage } from '@/entities/image'
import { type PostVideo } from '@/entities/video'
import { BlockList, PhotoGroup, type PhotoFit } from '@/entities/post'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
import { type CopyFallbackElement } from '@/shared/lib'
import { Button, FieldLabel, SegmentedControl, Textarea, TextField, Typography } from '@/shared/ui'
import { EXPORT_FORMATS } from '../config/guidance'
import { type CopyStatus } from '../model/copy-status'
import { type CopyTarget } from '../model/copy-target'
import { useExportPanel } from '../model/useExportPanel'
import { CaptionCopy } from './CaptionCopy'
import { PhotoCopy } from './PhotoCopy'
import { PreviewPhoto } from './PreviewPhoto'

interface ExportPanelProps {
  content: PostContent
  images: readonly PostImage[]
  /** The post's clips, so the Naver preview can play one in marker order (VIDEO-15). */
  videos?: readonly PostVideo[]
  createdAt: string
  contentLanguage: ContentLanguage
  /** Asks the owner of the post query for fresh photo URLs. Called when a preview photo fails to
   *  load, which — the panel outliving a presigned URL being the ordinary way that happens — is
   *  usually one refetch away from fixed. Optional so the panel stays renderable from a test or a
   *  surface that holds no query. */
  onPhotoUrlsStale?: () => void
}

/** Four synchronous browser-only derivations of the canonical block array. */
export function ExportPanel({
  content,
  images,
  videos = [],
  createdAt,
  contentLanguage,
  onPhotoUrlsStale,
}: ExportPanelProps) {
  const { t } = useTranslation('posts')
  const {
    format,
    selectFormat,
    hashtags,
    fallbackOutput,
    rawFieldVisible,
    status,
    hasPhotos,
    photoNumbersByBlock,
    imagesByFilename,
    outputRef,
    titleRef,
    tagsRef,
    copyButtonRef,
    copyWithTagsButtonRef,
    copyTitle,
    copyTags,
    copyOutput,
    copyOutputWithTags,
    captionCopy,
    copyCaption,
    registerCaptionField,
    photoCopied,
    photoFailure,
    copyPhoto,
  } = useExportPanel({ content, images, createdAt, contentLanguage })
  const formatOptions = EXPORT_FORMATS.map((value) => ({
    value,
    label: t(`export.formatLabel.${value}`),
  }))

  /** One status line's words: the copy's own confirmation, or the manual-selection hint. */
  const statusText = (kind: CopyStatus, copiedText: string) =>
    kind === 'copied' ? copiedText : kind === 'manual' ? t('export.manualCopy') : ''

  // One line per copy target, mounted whether or not it has anything to say: a live region has to
  // exist BEFORE its text changes or a screen reader announces nothing, and the sole confirmation
  // used to be a 1.5s label swap on the button under the thumb that hid it.
  const titleStatus = statusText(status.title, t('export.titleCopied'))
  const outputStatus = statusText(status.output, t('action.copied', { ns: 'common' }))
  const outputWithTagsStatus = statusText(status.outputWithTags, t('export.withTagsCopied'))
  const tagsStatus = statusText(status.tags, t('export.tagsCopied'))

  /** What one preview photo needs to carry its CAPTION's copy. Built here rather than inline at
   *  both call sites because the missing-photo branch has to agree with the ordinary one — a
   *  marker with no pixels still holds its number and still has a caption to copy. */
  function captionCopyProps(block: PostContent['blocks'][number], index: number) {
    // A group's caption is keyed by its first photo's number: unique per caption all the same.
    const marker = photoNumbersByBlock.get(index)?.[0] ?? 0
    const caption = captionCopy(marker, block.caption)
    return {
      marker,
      captionStatus: statusText(caption.status, t('export.captionCopied')),
      captionFellBack: caption.fellBack,
      onCopyCaption: () => void copyCaption(marker, block.caption),
      registerCaptionField: (element: CopyFallbackElement | null) =>
        registerCaptionField(marker, element),
    }
  }

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
        onChange={selectFormat}
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
                  and must be the thing that gives way, never the control beside it (THEME-32). */}
              <TextField
                id="export-title"
                ref={titleRef}
                value={content.title}
                readOnly
                className="min-w-0 flex-1"
              />
              <Button variant="secondary" className="shrink-0" onClick={() => void copyTitle()}>
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

        {/* Outside the `naver` branch on purpose: the site HTML and the Markdown front matter
            embed their tags as markup, and a ready-to-paste string is a different artifact worth
            having on every tab. Mounted only when there is something to paste — an empty tag list
            gets no field and no label (THEME-29). Field order reads title → tags → output. */}
        {hashtags && (
          <div className="mt-4">
            <FieldLabel htmlFor="export-tags">{t('export.tags')}</FieldLabel>
            <div className="mt-2 flex items-center gap-2">
              {/* `min-w-0` on the field, `shrink-0` on the button, exactly as the title above:
                  the tags are model output and must be the thing that gives way (THEME-32). */}
              <TextField
                id="export-tags"
                ref={tagsRef}
                value={hashtags}
                readOnly
                className="min-w-0 flex-1"
              />
              <Button
                variant="secondary"
                className="shrink-0"
                // The field is always mounted when this renders, so `copyText` gets a real
                // fallback element and selects it itself — no reveal effect, unlike the Naver body.
                onClick={() => void copyTags()}
              >
                {t('export.copyTags')}
              </Button>
            </div>
            <Typography
              variant="body"
              as="p"
              role="status"
              className="text-content-tertiary mt-1 min-h-5"
            >
              {tagsStatus}
            </Typography>
          </div>
        )}

        {/* The copy action sits ABOVE the output, not after it. This panel renders inside the
            editor, which already docks its own bar, and two docked bars in one scroller stick to
            the same offset and paint over each other — so THEME-24's other option applies: put the
            action where the user already is. Above the field it shares a screen with the format
            tabs and the first lines of the result, which is what the user checks before copying;
            after an autoGrow field it would be a whole post's length away. */}
        <div className="mt-4 flex flex-col gap-2 sm:flex-row-reverse sm:items-center sm:justify-end sm:gap-3">
          {/* The label stays 복사: a swap to 복사됨 resizes the target under the thumb that just
              pressed it, and it was also the only signal that anything had happened (THEME-28). */}
          <div className="flex w-full gap-2 sm:w-auto">
            <Button
              ref={copyButtonRef}
              variant="cta"
              className="min-w-0 flex-1 sm:flex-none"
              onClick={() => void copyOutput()}
            >
              {t('action.copy', { ns: 'common' })}
            </Button>
            {format === 'naver' && hashtags && (
              <Button
                ref={copyWithTagsButtonRef}
                variant="secondary"
                className="min-w-0 flex-1 sm:flex-none"
                onClick={() => void copyOutputWithTags()}
              >
                {t('export.copyWithTags')}
              </Button>
            )}
          </div>
          {/* Always mounted, never conditionally inserted: a live region that first appears WITH
              its text already in it is not announced (THEME-33). */}
          <Typography variant="body" as="p" role="status" className="text-content-tertiary min-h-5">
            {outputStatus || outputWithTagsStatus}
          </Typography>
        </div>

        {rawFieldVisible ? (
          <>
            <FieldLabel htmlFor="export-output" className="sr-only">
              {t('export.result')}
            </FieldLabel>
            {/* A Korean post runs to ~58 lines at this width. A fixed 18-row box scrolled
                internally, so every vertical swipe that landed on it moved the output instead of
                the page and the only place left to scroll was the 16px gutter (THEME-25) — it grows
                instead. No mono face: Tailwind's stock mono stack carries no Hangul, so every
                glyph fell back (THEME-19). */}
            <Textarea
              id="export-output"
              ref={outputRef}
              value={fallbackOutput}
              readOnly
              spellCheck={false}
              rows={8}
              autoGrow
              className="mt-3"
            />
          </>
        ) : (
          /* The Naver preview IS the post (EXPORT-9): what SmartEditor gets is plain text with
             `사진_<n>_…_사진` markers, but what the human reads here is the rendering they are about to
             publish — photos inline at their marker positions, each carrying its own copy. The
             header is suppressed because the body copy does not paste it; the title has its own
             field above. */
          <BlockList
            content={content}
            images={images}
            videos={videos}
            label={t('export.preview')}
            className="mt-3 pb-0"
            renderHeader={() => null}
            // A clip plays inline and offers NO copy control: the clipboard cannot carry a video
            // file from a page, and the file the author filmed is on the device they are pasting
            // from — so the hint says to attach the original there (VIDEO-15).
            renderVideo={(_block, rendered) => (
              <div>
                {rendered}
                <Typography variant="meta" as="p" className="text-content-secondary mt-1">
                  {t('export.videoHint')}
                </Typography>
              </div>
            )}
            renderBlock={(block, index, rendered) => {
              if (block.type === BlockType.GALLERY) {
                // One place with one caption (EXPORT-26): every photo its own copy control named
                // by its own number, and the group's caption one control under it.
                const numbers = photoNumbersByBlock.get(index) ?? []
                const copyProps = captionCopyProps(block, index)
                const photoCell = (file: string, position: number, fit: PhotoFit) => {
                  const number = numbers[position] ?? 0
                  const target: CopyTarget = `photo:${number}:${file}`
                  const image = imagesByFilename.get(file)
                  return (
                    <PhotoCopy
                      file={file}
                      alt={block.alt}
                      image={image}
                      marker={number}
                      copied={image ? photoCopied(target) : false}
                      failure={image ? photoFailure(target) : undefined}
                      onCopy={(element, rotation) => void copyPhoto(target, element, rotation)}
                      onStale={onPhotoUrlsStale}
                      fit={fit}
                    />
                  )
                }
                return (
                  <PhotoGroup
                    block={block}
                    images={imagesByFilename}
                    renderPhoto={(file, position, _image, fit) => photoCell(file, position, fit)}
                    renderMissingPhoto={photoCell}
                    renderCaption={() => (
                      <CaptionCopy
                        caption={block.caption}
                        ariaLabel={t('export.groupCaptionCopyAria', {
                          numbers: numbers.map((number) => number + 1).join(', '),
                        })}
                        captionStatus={copyProps.captionStatus}
                        captionFellBack={copyProps.captionFellBack}
                        onCopyCaption={copyProps.onCopyCaption}
                        registerCaptionField={copyProps.registerCaptionField}
                      />
                    )}
                  />
                )
              }
              if (block.type !== BlockType.IMAGE) return rendered
              const copyProps = captionCopyProps(block, index)
              const target: CopyTarget = `photo:${copyProps.marker}:${block.file}`
              return (
                <PreviewPhoto
                  file={block.file}
                  alt={block.alt}
                  caption={block.caption}
                  image={imagesByFilename.get(block.file)}
                  copied={photoCopied(target)}
                  failure={photoFailure(target)}
                  onCopy={(element, rotation) => void copyPhoto(target, element, rotation)}
                  onStale={onPhotoUrlsStale}
                  {...copyProps}
                />
              )
            }}
            // A marker whose photo is missing keeps its POSITION: dropping it would shift every
            // later photo against its marker — the one thing marker order exists to prevent.
            renderMissingImage={(block, index) => {
              const copyProps = captionCopyProps(block, index)
              return (
                <PreviewPhoto
                  file={block.file}
                  alt=""
                  caption={block.caption}
                  image={undefined}
                  copied={false}
                  failure={undefined}
                  onCopy={() => undefined}
                  onStale={onPhotoUrlsStale}
                  {...copyProps}
                />
              )
            }}
          />
        )}
      </div>
    </section>
  )
}
