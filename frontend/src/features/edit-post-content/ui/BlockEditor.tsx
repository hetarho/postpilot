import { clone, create } from '@bufbuild/protobuf'
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { BlockList, type PostDraft } from '@/entities/post'
import { BlockSchema, BlockType, PostContentSchema, type Block, type PostContent } from '@/shared/api'
import { Button, Editable, FieldLabel, FieldMessage, Select, Textarea, TextField } from '@/shared/ui'
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
  const [content, setContent] = useState(() => clone(PostContentSchema, post.content!))
  const valid = useMemo(
    () => validContent(content, post.images.map((image) => image.filename)),
    [content, post.images],
  )
  const autosave = useContentAutosave({
    slug: post.slug,
    revision: post.contentRevision,
    content,
    valid,
  })
  useEffect(() => onContentChange?.(content), [content, onContentChange])
  useImperativeHandle(ref, () => ({ flush: autosave.flush, content: () => content }), [autosave.flush, content])

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
        <h2 id="content-editor-heading" className="text-lg font-semibold tracking-tight">
          글 다듬기
        </h2>
        <p role="status" className="text-content-tertiary text-sm">
          {saveLabel(autosave.state)}
        </p>
      </div>
      {autosave.state === 'conflict' && (
        <FieldMessage className="mt-2">
          다른 화면에서 글이 바뀌었어요. 이 화면을 새로고침한 뒤 다시 수정해 주세요.
        </FieldMessage>
      )}
      {!valid && <FieldMessage className="mt-2">빈 블록을 채워 주세요.</FieldMessage>}

      <BlockList
        content={content}
        images={post.images}
        renderHeader={(rendered) => (
          <Editable
            editLabel="제목과 요약, 태그 수정"
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
        문단 추가
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
  return (
    <Editable
      editLabel={`${index + 1}번째 블록 수정`}
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
  // Captured on mount — the moment the block entered edit mode — because the edits write through
  // to the content as they are typed. This ref IS the saved value 취소 restores.
  const opened = useRef(block)
  return (
    <article className="bg-surface-raised rounded-lg p-4">
      <div className="flex flex-wrap items-center gap-2">
        <FieldLabel htmlFor={`block-type-${index}`} className="sr-only">
          {index + 1}번째 블록 종류
        </FieldLabel>
        <Select
          id={`block-type-${index}`}
          value={block.type}
          onChange={(event) => onChange(freshBlock(Number(event.target.value) as BlockType, filenames[0]))}
          className="w-auto min-w-32"
        >
          <option value={BlockType.TEXT}>문단</option>
          <option value={BlockType.HEADING}>소제목</option>
          <option value={BlockType.QUOTE}>인용</option>
          <option value={BlockType.LIST}>목록</option>
          {filenames.length > 0 && <option value={BlockType.IMAGE}>사진</option>}
        </Select>
        <span className="ml-auto flex gap-1">
          {/* Leaving edit mode is part of moving, for the same reason as deleting: the rows are
              keyed by position, so a mount-time snapshot left open over the block that shifted into
              this slot would overwrite THAT block on 취소. */}
          <Button variant="ghost" aria-label={`${index + 1}번째 블록 위로`} disabled={index === 0} onClick={() => { onMove(-1); onDone() }}>↑</Button>
          <Button variant="ghost" aria-label={`${index + 1}번째 블록 아래로`} disabled={index === blockCount - 1} onClick={() => { onMove(1); onDone() }}>↓</Button>
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
            삭제
          </Button>
        </span>
      </div>
      <BlockFields block={block} index={index} filenames={filenames} onChange={onChange} />
      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onDone}>저장</Button>
        <Button
          variant="ghost"
          onClick={() => {
            onChange(opened.current)
            onDone()
          }}
        >
          취소
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
  // Only the three fields this editor owns. Snapshotting the whole PostContent would make 취소
  // revert block edits made while the header happened to be open — the two editors are independent.
  const opened = useRef({ title: content.title, summary: content.summary, tags: [...content.tags] })
  return (
    <div className="bg-surface-raised mt-12 grid gap-4 rounded-lg p-4">
      <div>
        <FieldLabel htmlFor="generated-title">본문 제목</FieldLabel>
        <TextField
          id="generated-title"
          value={content.title}
          onChange={(event) => onChange(create(PostContentSchema, { ...content, title: event.target.value }))}
          className="mt-1"
        />
      </div>
      <div>
        <FieldLabel htmlFor="generated-summary">요약</FieldLabel>
        <Textarea
          id="generated-summary"
          rows={3}
          autoGrow
          value={content.summary}
          onChange={(event) => onChange(create(PostContentSchema, { ...content, summary: event.target.value }))}
          className="max-h-field mt-1"
        />
      </div>
      <div>
        <FieldLabel htmlFor="generated-tags">태그</FieldLabel>
        <TextField
          id="generated-tags"
          value={content.tags.join(', ')}
          onChange={(event) =>
            onChange(
              create(PostContentSchema, {
                ...content,
                tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean),
              }),
            )
          }
          placeholder="여행, 카페"
          className="mt-1"
        />
      </div>
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onDone}>저장</Button>
        <Button
          variant="ghost"
          onClick={() => {
            onChange(create(PostContentSchema, { ...content, ...opened.current }))
            onDone()
          }}
        >
          취소
        </Button>
      </div>
    </div>
  )
}

function BlockFields({ block, index, filenames, onChange }: { block: Block; index: number; filenames: string[]; onChange: (block: Block) => void }) {
  if (block.type === BlockType.IMAGE) {
    return (
      <div className="mt-3 grid gap-3">
        <FieldLabel htmlFor={`block-image-${index}`}>첨부 사진</FieldLabel>
        <Select id={`block-image-${index}`} value={block.file} onChange={(event) => onChange(create(BlockSchema, { ...block, file: event.target.value }))}>
          {filenames.map((filename) => <option key={filename} value={filename}>{filename}</option>)}
        </Select>
        <TextField aria-label="대체 텍스트" value={block.alt} onChange={(event) => onChange(create(BlockSchema, { ...block, alt: event.target.value }))} placeholder="사진 설명" />
        <TextField aria-label="사진 캡션" value={block.caption} onChange={(event) => onChange(create(BlockSchema, { ...block, caption: event.target.value }))} placeholder="캡션 (선택)" />
      </div>
    )
  }
  if (block.type === BlockType.LIST) {
    return <Textarea aria-label={`${index + 1}번째 목록, 한 줄에 한 항목`} rows={3} autoGrow value={block.items.join('\n')} onChange={(event) => onChange(create(BlockSchema, { ...block, items: event.target.value.split('\n') }))} className="mt-3" />
  }
  return (
    <div className="mt-3 grid gap-3">
      {block.type === BlockType.HEADING && (
        <Select aria-label="제목 단계" value={block.level} onChange={(event) => onChange(create(BlockSchema, { ...block, level: Number(event.target.value) }))}>
          {[1, 2, 3, 4, 5, 6].map((level) => <option key={level} value={level}>제목 {level}</option>)}
        </Select>
      )}
      <Textarea aria-label={`${index + 1}번째 블록 내용`} rows={block.type === BlockType.HEADING ? 1 : 3} autoGrow value={block.content} onChange={(event) => onChange(create(BlockSchema, { ...block, content: event.target.value }))} />
    </div>
  )
}

function freshBlock(type: BlockType, firstImage?: string): Block {
  switch (type) {
    case BlockType.HEADING: return create(BlockSchema, { type, level: 2, content: '새 소제목' })
    case BlockType.QUOTE: return create(BlockSchema, { type, content: '새 인용문' })
    case BlockType.LIST: return create(BlockSchema, { type, items: ['새 항목'] })
    case BlockType.IMAGE: return create(BlockSchema, { type, file: firstImage ?? '' })
    default: return create(BlockSchema, { type: BlockType.TEXT, content: '새 문단' })
  }
}

function validContent(content: PostContent, filenames: string[]): boolean {
  if (content.blocks.length === 0) return false
  return content.blocks.every((block) => {
    if (block.type === BlockType.IMAGE) return filenames.includes(block.file)
    if (block.type === BlockType.LIST) return block.items.length > 0 && block.items.every((item) => item.trim() !== '')
    if (block.type === BlockType.HEADING) return block.content.trim() !== '' && block.level >= 1 && block.level <= 6
    return (block.type === BlockType.TEXT || block.type === BlockType.QUOTE) && block.content.trim() !== ''
  })
}

function saveLabel(state: ReturnType<typeof useContentAutosave>['state']) {
  return { idle: '', dirty: '저장 대기 중', saving: '저장 중…', saved: '저장됨', error: '저장하지 못했어요', conflict: '수정 충돌' }[state]
}
