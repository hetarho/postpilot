import { clone, create } from '@bufbuild/protobuf'
import { forwardRef, useEffect, useImperativeHandle, useMemo, useState, type ReactNode } from 'react'
import type { PostDraft } from '@/entities/post'
import { BlockSchema, BlockType, PostContentSchema, type Block, type PostContent } from '@/shared/api'
import { POST_TARGET_LENGTH_DEFAULT, POST_TARGET_LENGTH_MAX, POST_TARGET_LENGTH_MIN } from '@/shared/config'
import { Button, FieldLabel, FieldMessage, Select, Textarea, TextField } from '@/shared/ui'
import { useContentAutosave } from '../model/useContentAutosave'

export interface BlockEditorHandle {
  flush: () => Promise<void>
  content: () => PostContent
}

export const BlockEditor = forwardRef<BlockEditorHandle, { post: PostDraft; onContentChange?: (content: PostContent) => void; renderSentenceAction?: (text: string, flush: () => Promise<void>) => ReactNode }>(function BlockEditor(
  { post, onContentChange, renderSentenceAction },
  ref,
) {
  const [content, setContent] = useState(() => clone(PostContentSchema, post.content!))
  const [targetLength, setTargetLength] = useState(post.targetLength || POST_TARGET_LENGTH_DEFAULT)
  const valid = useMemo(
    () =>
      targetLength >= POST_TARGET_LENGTH_MIN &&
      targetLength <= POST_TARGET_LENGTH_MAX &&
      validContent(content, post.images.map((image) => image.filename)),
    [content, post.images, targetLength],
  )
  const autosave = useContentAutosave({
    slug: post.slug,
    revision: post.contentRevision,
    content,
    targetLength,
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
      {!valid && <FieldMessage className="mt-2">빈 블록을 채우고 목표 글자 수 범위를 확인해 주세요.</FieldMessage>}

      <h3 className="mt-5 text-xl font-semibold tracking-tight">{content.title}</h3>

      <div className="mt-4 grid gap-4">
        <div>
          <FieldLabel htmlFor="generated-title">본문 제목</FieldLabel>
          <TextField
            id="generated-title"
            value={content.title}
            onChange={(event) => setContent(create(PostContentSchema, { ...content, title: event.target.value }))}
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
            onChange={(event) => setContent(create(PostContentSchema, { ...content, summary: event.target.value }))}
            className="mt-1"
          />
        </div>
        <div>
          <FieldLabel htmlFor="generated-tags">태그</FieldLabel>
          <TextField
            id="generated-tags"
            value={content.tags.join(', ')}
            onChange={(event) =>
              setContent(
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
        <div>
          <FieldLabel htmlFor="content-target-length">목표 글자 수</FieldLabel>
          <TextField
            id="content-target-length"
            type="number"
            min={POST_TARGET_LENGTH_MIN}
            max={POST_TARGET_LENGTH_MAX}
            value={targetLength}
            onChange={(event) => setTargetLength(Number(event.target.value))}
            className="mt-1"
          />
        </div>
      </div>

      <div className="mt-6 space-y-4">
        {content.blocks.map((block, index) => (
          <article key={`${index}-${block.type}`} className="bg-surface-raised rounded-lg p-4">
            <div className="flex flex-wrap items-center gap-2">
              <FieldLabel htmlFor={`block-type-${index}`} className="sr-only">
                {index + 1}번째 블록 종류
              </FieldLabel>
              <Select
                id={`block-type-${index}`}
                value={block.type}
                onChange={(event) => updateBlock(index, freshBlock(Number(event.target.value) as BlockType, post.images[0]?.filename))}
                className="w-auto min-w-32"
              >
                <option value={BlockType.TEXT}>문단</option>
                <option value={BlockType.HEADING}>소제목</option>
                <option value={BlockType.QUOTE}>인용</option>
                <option value={BlockType.LIST}>목록</option>
                {post.images.length > 0 && <option value={BlockType.IMAGE}>사진</option>}
              </Select>
              <span className="ml-auto flex gap-1">
                <Button variant="ghost" aria-label={`${index + 1}번째 블록 위로`} disabled={index === 0} onClick={() => moveBlock(index, -1)}>↑</Button>
                <Button variant="ghost" aria-label={`${index + 1}번째 블록 아래로`} disabled={index === content.blocks.length - 1} onClick={() => moveBlock(index, 1)}>↓</Button>
                <Button variant="ghost" disabled={content.blocks.length === 1} onClick={() => removeBlock(index)}>삭제</Button>
              </span>
            </div>
            <BlockFields block={block} index={index} filenames={post.images.map((image) => image.filename)} onChange={(value) => updateBlock(index, value)} />
            {(block.type === BlockType.TEXT || block.type === BlockType.QUOTE) && renderSentenceAction?.(block.content, autosave.flush)}
          </article>
        ))}
      </div>
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
