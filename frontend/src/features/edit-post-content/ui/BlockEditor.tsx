import { clone, create } from '@bufbuild/protobuf'
import i18next from 'i18next'
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { BlockList, type PostDraft } from '@/entities/post'
import {
  BlockSchema,
  BlockType,
  PostContentSchema,
  type Block,
  type PostContent,
} from '@/shared/api'
import {
  Button,
  Editable,
  FieldLabel,
  FieldMessage,
  Select,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import { useContentAutosave } from '../model/useContentAutosave'

export interface BlockEditorHandle {
  flush: () => Promise<bigint>
  content: () => PostContent
}

/** The draft, read first. `BlockList` renders it as prose — the reading view plan 06 specified —
 *  and each block carries one edit control that swaps just that block for the controls it has
 *  always had.
 *
 *  Edit mode writes THROUGH to the content, so the autosave keeps running on every keystroke
 *  exactly as it did when every block was a permanent form: an edit in progress is never text the
 *  app is holding hostage. 취소 restores the value the block had when its editor opened, which is
 *  the snapshot `BlockEditRow` captures on mount. */
export const BlockEditor = forwardRef<
  BlockEditorHandle,
  {
    post: PostDraft
    onContentChange?: (content: PostContent) => void
    renderSentenceAction?: (text: string, flush: () => Promise<bigint>) => ReactNode
  }
>(function BlockEditor({ post, onContentChange, renderSentenceAction }, ref) {
  const { t } = useTranslation('posts')
  const [content, setContent] = useState(() => clone(PostContentSchema, post.content!))
  const valid = useMemo(
    () =>
      validContent(
        content,
        post.images.map((image) => image.filename),
      ),
    [content, post.images],
  )
  const autosave = useContentAutosave({
    slug: post.slug,
    revision: post.contentRevision,
    content,
    valid,
  })
  useEffect(() => onContentChange?.(content), [content, onContentChange])
  useImperativeHandle(ref, () => ({ flush: autosave.flush, content: () => content }), [
    autosave.flush,
    content,
  ])

  const updateBlock = (index: number, value: Block) => {
    const next = clone(PostContentSchema, content)
    next.blocks[index] = value
    setContent(next)
  }
  const removeBlock = (index: number) => {
    if (content.blocks.length === 1) return
    const next = clone(PostContentSchema, content)
    next.blocks.splice(index, 1)
    setContent(next)
  }
  const moveBlock = (index: number, direction: -1 | 1) => {
    const destination = index + direction
    if (destination < 0 || destination >= content.blocks.length) return
    const next = clone(PostContentSchema, content)
    ;[next.blocks[index], next.blocks[destination]] = [next.blocks[destination], next.blocks[index]]
    setContent(next)
  }

  return (
    <section aria-labelledby="content-editor-heading" className="mt-10">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <Typography variant="title" id="content-editor-heading">
          {t('edit.refine')}
        </Typography>
        <Typography variant="body" role="status" className="text-content-tertiary">
          {saveLabel(autosave.state)}
        </Typography>
      </div>
      {autosave.state === 'conflict' && (
        <FieldMessage className="mt-2">{t('edit.conflict')}</FieldMessage>
      )}
      {!valid && <FieldMessage className="mt-2">{t('edit.emptyBlock')}</FieldMessage>}

      <BlockList
        content={content}
        images={post.images}
        renderHeader={(rendered) => (
          <Editable
            editLabel={t('edit.titleSummaryTags')}
            edit={(exit) => <HeaderFields content={content} onChange={setContent} onDone={exit} />}
          >
            {rendered}
          </Editable>
        )}
        renderBlock={(block, index, rendered) => (
          <BlockEditRow
            block={block}
            index={index}
            blockCount={content.blocks.length}
            filenames={post.images.map((image) => image.filename)}
            onChange={(value) => updateBlock(index, value)}
            onMove={(direction) => moveBlock(index, direction)}
            onRemove={() => removeBlock(index)}
          >
            {rendered}
            {(block.type === BlockType.TEXT || block.type === BlockType.QUOTE) &&
              renderSentenceAction?.(block.content, autosave.flush)}
          </BlockEditRow>
        )}
      />

      <Button
        variant="secondary"
        className="mt-4 w-full sm:w-auto"
        onClick={() => {
          const next = clone(PostContentSchema, content)
          next.blocks.push(freshBlock(BlockType.TEXT))
          setContent(next)
        }}
      >
        {t('edit.addParagraph')}
      </Button>
    </section>
  )
})

function BlockEditRow({
  block,
  index,
  blockCount,
  filenames,
  onChange,
  onMove,
  onRemove,
  children,
}: {
  block: Block
  index: number
  blockCount: number
  filenames: string[]
  onChange: (block: Block) => void
  onMove: (direction: -1 | 1) => void
  onRemove: () => void
  children: ReactNode
}) {
  const { t } = useTranslation('posts')
  return (
    <Editable
      editLabel={t('edit.blockEdit', { index: index + 1 })}
      edit={(exit) => (
        <BlockControls
          block={block}
          index={index}
          blockCount={blockCount}
          filenames={filenames}
          onChange={onChange}
          onMove={onMove}
          onRemove={onRemove}
          onDone={exit}
        />
      )}
    >
      {children}
    </Editable>
  )
}

function BlockControls({
  block,
  index,
  blockCount,
  filenames,
  onChange,
  onMove,
  onRemove,
  onDone,
}: {
  block: Block
  index: number
  blockCount: number
  filenames: string[]
  onChange: (block: Block) => void
  onMove: (direction: -1 | 1) => void
  onRemove: () => void
  onDone: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  // Captured on mount — the moment the block entered edit mode — because the edits write through
  // to the content as they are typed. This ref IS the saved value 취소 restores.
  const opened = useRef(block)
  return (
    <article className="bg-surface-raised rounded-lg p-4">
      <div className="flex flex-wrap items-center gap-2">
        <FieldLabel htmlFor={`block-type-${index}`} className="sr-only">
          {t('edit.blockType', { ns: 'posts', index: index + 1 })}
        </FieldLabel>
        <Select
          id={`block-type-${index}`}
          value={block.type}
          onChange={(event) =>
            onChange(freshBlock(Number(event.target.value) as BlockType, filenames[0]))
          }
          className="w-auto min-w-32"
        >
          <option value={BlockType.TEXT}>{t('edit.blockTypeOption.text', { ns: 'posts' })}</option>
          <option value={BlockType.HEADING}>
            {t('edit.blockTypeOption.heading', { ns: 'posts' })}
          </option>
          <option value={BlockType.QUOTE}>
            {t('edit.blockTypeOption.quote', { ns: 'posts' })}
          </option>
          <option value={BlockType.LIST}>{t('edit.blockTypeOption.list', { ns: 'posts' })}</option>
          {filenames.length > 0 && (
            <option value={BlockType.IMAGE}>
              {t('edit.blockTypeOption.image', { ns: 'posts' })}
            </option>
          )}
        </Select>
        <span className="ml-auto flex gap-1">
          {/* Leaving edit mode is part of moving, for the same reason as deleting: the rows are
              keyed by position, so a mount-time snapshot left open over the block that shifted into
              this slot would overwrite THAT block on 취소. */}
          <Button
            variant="ghost"
            aria-label={t('edit.moveUp', { ns: 'posts', index: index + 1 })}
            disabled={index === 0}
            onClick={() => {
              onMove(-1)
              onDone()
            }}
          >
            ↑
          </Button>
          <Button
            variant="ghost"
            aria-label={t('edit.moveDown', { ns: 'posts', index: index + 1 })}
            disabled={index === blockCount - 1}
            onClick={() => {
              onMove(1)
              onDone()
            }}
          >
            ↓
          </Button>
          {/* Leaving edit mode is part of deleting: the rows are keyed by position, so without it
              the open editor would stay mounted over whichever block shifted up into this slot. */}
          <Button
            variant="ghost"
            disabled={blockCount === 1}
            onClick={() => {
              onRemove()
              onDone()
            }}
          >
            {t('action.delete', { ns: 'common' })}
          </Button>
        </span>
      </div>
      <BlockFields block={block} index={index} filenames={filenames} onChange={onChange} />
      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onDone}>
          {t('action.save', { ns: 'common' })}
        </Button>
        <Button
          variant="ghost"
          onClick={() => {
            onChange(opened.current)
            onDone()
          }}
        >
          {t('action.cancel', { ns: 'common' })}
        </Button>
      </div>
    </article>
  )
}

function HeaderFields({
  content,
  onChange,
  onDone,
}: {
  content: PostContent
  onChange: (content: PostContent) => void
  onDone: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  // Only the three fields this editor owns. Snapshotting the whole PostContent would make 취소
  // revert block edits made while the header happened to be open — the two editors are independent.
  const opened = useRef({ title: content.title, summary: content.summary, tags: [...content.tags] })
  return (
    <div className="bg-surface-raised mt-12 grid gap-4 rounded-lg p-4">
      <div>
        <FieldLabel htmlFor="generated-title">{t('edit.bodyTitle', { ns: 'posts' })}</FieldLabel>
        <TextField
          id="generated-title"
          value={content.title}
          onChange={(event) =>
            onChange(create(PostContentSchema, { ...content, title: event.target.value }))
          }
          className="mt-1"
        />
      </div>
      <div>
        <FieldLabel htmlFor="generated-summary">{t('edit.summary', { ns: 'posts' })}</FieldLabel>
        <Textarea
          id="generated-summary"
          rows={3}
          autoGrow
          value={content.summary}
          onChange={(event) =>
            onChange(create(PostContentSchema, { ...content, summary: event.target.value }))
          }
          className="max-h-field mt-1"
        />
      </div>
      <div>
        <FieldLabel htmlFor="generated-tags">{t('tags', { ns: 'posts' })}</FieldLabel>
        <TextField
          id="generated-tags"
          value={content.tags.join(', ')}
          onChange={(event) =>
            onChange(
              create(PostContentSchema, {
                ...content,
                tags: event.target.value
                  .split(',')
                  .map((tag) => tag.trim())
                  .filter(Boolean),
              }),
            )
          }
          placeholder={t('edit.tagPlaceholder', { ns: 'posts' })}
          className="mt-1"
        />
      </div>
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onDone}>
          {t('action.save', { ns: 'common' })}
        </Button>
        <Button
          variant="ghost"
          onClick={() => {
            onChange(create(PostContentSchema, { ...content, ...opened.current }))
            onDone()
          }}
        >
          {t('action.cancel', { ns: 'common' })}
        </Button>
      </div>
    </div>
  )
}

function BlockFields({
  block,
  index,
  filenames,
  onChange,
}: {
  block: Block
  index: number
  filenames: string[]
  onChange: (block: Block) => void
}) {
  const { t } = useTranslation('posts')
  if (block.type === BlockType.IMAGE) {
    return (
      <div className="mt-3 grid gap-3">
        <FieldLabel htmlFor={`block-image-${index}`}>{t('edit.attachedPhoto')}</FieldLabel>
        <Select
          id={`block-image-${index}`}
          value={block.file}
          onChange={(event) =>
            onChange(create(BlockSchema, { ...block, file: event.target.value }))
          }
        >
          {filenames.map((filename) => (
            <option key={filename} value={filename}>
              {filename}
            </option>
          ))}
        </Select>
        <TextField
          aria-label={t('edit.altText')}
          value={block.alt}
          onChange={(event) => onChange(create(BlockSchema, { ...block, alt: event.target.value }))}
          placeholder={t('edit.photoDescription')}
        />
        <TextField
          aria-label={t('edit.caption')}
          value={block.caption}
          onChange={(event) =>
            onChange(create(BlockSchema, { ...block, caption: event.target.value }))
          }
          placeholder={t('edit.captionPlaceholder')}
        />
      </div>
    )
  }
  if (block.type === BlockType.LIST) {
    return (
      <Textarea
        aria-label={t('edit.listLabel', { index: index + 1 })}
        rows={3}
        autoGrow
        value={block.items.join('\n')}
        onChange={(event) =>
          onChange(create(BlockSchema, { ...block, items: event.target.value.split('\n') }))
        }
        className="mt-3"
      />
    )
  }
  return (
    <div className="mt-3 grid gap-3">
      {block.type === BlockType.HEADING && (
        <Select
          aria-label={t('edit.headingLevel')}
          value={block.level}
          onChange={(event) =>
            onChange(create(BlockSchema, { ...block, level: Number(event.target.value) }))
          }
        >
          {[1, 2, 3, 4, 5, 6].map((level) => (
            <option key={level} value={level}>
              {t('edit.headingOption', { level })}
            </option>
          ))}
        </Select>
      )}
      <Textarea
        aria-label={t('edit.blockContent', { index: index + 1 })}
        rows={block.type === BlockType.HEADING ? 1 : 3}
        autoGrow
        value={block.content}
        onChange={(event) =>
          onChange(create(BlockSchema, { ...block, content: event.target.value }))
        }
      />
    </div>
  )
}

function freshBlock(type: BlockType, firstImage?: string): Block {
  switch (type) {
    case BlockType.HEADING:
      return create(BlockSchema, {
        type,
        level: 2,
        content: i18next.t('edit.newBlock.heading', { ns: 'posts' }),
      })
    case BlockType.QUOTE:
      return create(BlockSchema, {
        type,
        content: i18next.t('edit.newBlock.quote', { ns: 'posts' }),
      })
    case BlockType.LIST:
      return create(BlockSchema, {
        type,
        items: [i18next.t('edit.newBlock.list', { ns: 'posts' })],
      })
    case BlockType.IMAGE:
      return create(BlockSchema, { type, file: firstImage ?? '' })
    default:
      return create(BlockSchema, {
        type: BlockType.TEXT,
        content: i18next.t('edit.newBlock.text', { ns: 'posts' }),
      })
  }
}

function validContent(content: PostContent, filenames: string[]): boolean {
  if (content.blocks.length === 0) return false
  return content.blocks.every((block) => {
    if (block.type === BlockType.IMAGE) return filenames.includes(block.file)
    if (block.type === BlockType.LIST)
      return block.items.length > 0 && block.items.every((item) => item.trim() !== '')
    if (block.type === BlockType.HEADING)
      return block.content.trim() !== '' && block.level >= 1 && block.level <= 6
    return (
      (block.type === BlockType.TEXT || block.type === BlockType.QUOTE) &&
      block.content.trim() !== ''
    )
  })
}

function saveLabel(state: ReturnType<typeof useContentAutosave>['state']) {
  switch (state) {
    case 'dirty':
      return i18next.t('state.savePending', { ns: 'common' })
    case 'saving':
      return i18next.t('action.saving', { ns: 'common' })
    case 'saved':
      return i18next.t('state.saved', { ns: 'common' })
    case 'error':
      return i18next.t('state.saveFailed', { ns: 'common' })
    case 'conflict':
      return i18next.t('state.conflict', { ns: 'common' })
    default:
      return ''
  }
}
