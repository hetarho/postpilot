import { useState, type DragEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ArrowRightLeft } from 'lucide-react'
import { Thumbnail } from '@/entities/image'
import type { PostStorylineParagraph } from '@/entities/post'
import { VideoTile } from '@/entities/video'
import { Button, Editable, Menu, Textarea, Typography, typographyStyles } from '@/shared/ui'
import { withFileIn, withText, withoutFile } from '../model/storyline-edits'

/** What a paragraph shows for one of its files. */
export interface StorylineAttachment {
  filename: string
  kind: 'photo' | 'video'
  /** A `blob:` URL right after an upload is fine; absent shows the filename. */
  viewUrl?: string
  width?: number
  height?: number
  durationMs?: number
  contentType?: string
}

/** The drag payload's type: a storyline file, never an arbitrary drop from outside the page. */
export const STORYLINE_FILE_TYPE = 'application/x-postpilot-storyline-file'

/** One paragraph of the storyline, editable by hand (POST-96): its number, its text edited in
 *  place, and its attachments as small tiles, each with a move control — the paragraph to move it
 *  to, or 빼기. From `sm:` a tile can also be dragged onto another paragraph. Every change hands
 *  the whole list back, which is what the draft queue saves. */
export function StorylineParagraphEditor({
  paragraphs,
  index,
  attachments,
  readOnly,
  onChange,
  onView,
}: {
  paragraphs: readonly PostStorylineParagraph[]
  index: number
  attachments: ReadonlyMap<string, StorylineAttachment>
  readOnly: boolean
  onChange: (paragraphs: PostStorylineParagraph[]) => void
  /** Opens one attachment large (POST-100). Viewing is not editing, so a read-only space has it. */
  onView?: (file: string) => void
}) {
  const { t } = useTranslation('posts')
  const paragraph = paragraphs[index]!
  const number = index + 1
  const [dropping, setDropping] = useState(false)

  const acceptsDrop = (event: DragEvent) =>
    !readOnly && event.dataTransfer.types.includes(STORYLINE_FILE_TYPE)

  const moveOptions = [
    ...paragraphs.map((_, at) => ({
      value: `p${at}`,
      label: t('storylineEdit.paragraph', { n: at + 1 }),
    })),
    { value: 'out', label: t('storylineEdit.takeOut') },
  ]

  return (
    <li
      className={clsx('rounded-md py-4', dropping && 'bg-row-bg-hover')}
      aria-label={t('storylineEdit.paragraph', { n: number })}
      onDragOver={(event) => {
        if (!acceptsDrop(event)) return
        event.preventDefault()
        setDropping(true)
      }}
      onDragLeave={() => setDropping(false)}
      onDrop={(event) => {
        setDropping(false)
        if (!acceptsDrop(event)) return
        event.preventDefault()
        const file = event.dataTransfer.getData(STORYLINE_FILE_TYPE)
        if (file) onChange(withFileIn(paragraphs, file, index))
      }}
    >
      <Typography variant="label" as="p" className="text-content-secondary">
        {t('storylineEdit.paragraph', { n: number })}
      </Typography>
      <Editable
        className="mt-1"
        readOnly={readOnly}
        editLabel={t('storylineEdit.editText', { n: number })}
        edit={(exit) => (
          <div className="flex flex-col gap-2">
            <Textarea
              autoGrow
              autoFocus
              rows={2}
              aria-label={t('storylineEdit.paragraph', { n: number })}
              value={paragraph.text}
              onChange={(event) => onChange(withText(paragraphs, index, event.currentTarget.value))}
            />
            <Button variant="secondary" className="self-end" onClick={exit}>
              {t('storylineEdit.done')}
            </Button>
          </div>
        )}
      >
        <Typography variant="body" className="text-content-primary whitespace-pre-wrap">
          {paragraph.text}
        </Typography>
      </Editable>
      {paragraph.files.length > 0 && (
        <ul className="mt-3 flex flex-wrap gap-3">
          {paragraph.files.map((file) => (
            <li
              key={file}
              className="flex flex-col items-start gap-1"
              draggable={!readOnly}
              onDragStart={(event) => {
                event.dataTransfer.setData(STORYLINE_FILE_TYPE, file)
                event.dataTransfer.effectAllowed = 'move'
              }}
            >
              <StorylineTile
                attachment={attachments.get(file)}
                filename={file}
                onView={onView && (() => onView(file))}
              />
              {!readOnly && (
                <Menu
                  label={t('storylineEdit.move', { file })}
                  value={`p${index}`}
                  options={moveOptions}
                  triggerIcon={<ArrowRightLeft aria-hidden="true" className="size-4" />}
                  onChange={(value) =>
                    onChange(
                      value === 'out'
                        ? withoutFile(paragraphs, file)
                        : withFileIn(paragraphs, file, Number(value.slice(1))),
                    )
                  }
                />
              )}
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

/** A small photo or clip tile; the filename stands in while there is no view URL. With `onView`
 *  the tile is a button that opens the attachment large (POST-100) — laid over the picture, inside
 *  the tile, and ringed inward because the tile clips anything drawn outside its corners. A tile
 *  with nothing to show is never that button. */
export function StorylineTile({
  attachment,
  filename,
  onView,
}: {
  attachment: StorylineAttachment | undefined
  filename: string
  onView?: () => void
}) {
  const { t } = useTranslation('posts')
  const missing = !attachment?.viewUrl
  const overlay = missing ? (
    <span
      className={typographyStyles({
        variant: 'meta',
        className: 'absolute inset-1 overflow-hidden break-all',
      })}
    >
      {filename}
    </span>
  ) : onView ? (
    <button
      type="button"
      aria-label={t('storylineEdit.view', { file: filename })}
      onClick={onView}
      className="absolute inset-0 cursor-zoom-in rounded-lg focus-visible:-outline-offset-2"
    />
  ) : undefined
  if (attachment?.kind === 'video')
    return (
      <VideoTile
        size="small"
        src={attachment.viewUrl}
        durationMs={attachment.durationMs ?? 0}
        contentType={attachment.contentType}
        controls={!onView}
      >
        {overlay}
      </VideoTile>
    )
  return (
    <Thumbnail
      size="small"
      src={attachment?.viewUrl}
      alt={filename}
      width={attachment?.width}
      height={attachment?.height}
    >
      {overlay}
    </Thumbnail>
  )
}
