import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { BlockSchema, BlockType, GalleryLayout, PostContentSchema } from '@/shared/api'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import type { OriginFieldLocator } from '../model/semantic-origin'
import { BlockList } from './BlockList'

it('renders the title metadata and all five canonical block types', () => {
  render(
    <BlockList
      content={POST_CONTENT_FIXTURE}
      images={[
        {
          id: 'image-1',
          filename: 'IMG_1.jpg',
          width: 1024,
          height: 768,
          bytes: 200_000,
          viewUrl: 'https://storage.test/IMG_1.jpg?signature=read',
        },
      ]}
    />,
  )

  expect(screen.getByRole('heading', { name: '비 온 뒤의 제주', level: 3 })).toHaveClass('text-xl')
  expect(screen.getByText('비가 그치기를 기다렸다.')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '바닷가로', level: 4 })).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '잔잔한 제주 바다' })).toHaveAttribute(
    'src',
    'https://storage.test/IMG_1.jpg?signature=read',
  )
  expect(screen.getByText('비 뒤의 바다')).toBeInTheDocument()
  expect(screen.getByText('서두르지 않아도 괜찮다.').closest('blockquote')).not.toBeNull()
  expect(screen.getByText('우산').closest('li')).not.toBeNull()
  // The fixture's second IMAGE has no matching image: with no `renderMissingImage` it renders
  // nothing, exactly as before the seam existed.
  expect(screen.queryByRole('img', { name: '구름 사이 햇빛' })).not.toBeInTheDocument()
})

it('hands an IMAGE block with no matching image to renderMissingImage, in place', () => {
  render(
    <BlockList
      content={POST_CONTENT_FIXTURE}
      images={[]}
      renderMissingImage={(block, index) => <p>{`missing ${block.file} at ${index}`}</p>}
    />,
  )

  expect(screen.getByText('missing IMG_1.jpg at 3')).toBeInTheDocument()
  expect(screen.getByText('missing IMG_2.jpg at 5')).toBeInTheDocument()
})

it('lets a consumer name the article apart from the reading view', () => {
  render(<BlockList content={POST_CONTENT_FIXTURE} images={[]} label="네이버 미리보기" />)
  expect(screen.getByRole('article', { name: '네이버 미리보기' })).toBeInTheDocument()
})

describe('a VIDEO block', () => {
  const clip = {
    id: 'video-1',
    filename: 'clip.mp4',
    width: 1920,
    height: 1080,
    bytes: 12_000_000,
    durationMs: 8_000,
    contentType: 'video/mp4',
    viewUrl: 'https://storage.test/clip.mp4?sig=read',
  }
  const content = create(PostContentSchema, {
    title: '제목',
    blocks: [
      create(BlockSchema, { type: BlockType.VIDEO, file: 'clip.mp4', caption: '파도' }),
      create(BlockSchema, { type: BlockType.VIDEO, file: 'gone.mp4' }),
    ],
  })

  // VIDEO-14: played in place, on demand. No autoplay and metadata only, so opening a draft with
  // three clips does not start pulling 600 MB.
  it('plays the matching clip with its caption, and never autoplays', () => {
    const { container } = render(<BlockList content={content} images={[]} videos={[clip]} />)

    const video = container.querySelector('video')
    expect(video).toHaveAttribute('src', clip.viewUrl)
    expect(video).toHaveAttribute('preload', 'metadata')
    expect(video).not.toHaveAttribute('autoplay')
    expect(video).toHaveAttribute('controls')
    expect(screen.getByText('파도')).toBeInTheDocument()
  })

  // The block still holds its position: a dropped one would shift everything after it against
  // what the export markers say.
  it('holds the position with the filename when no attached clip matches', () => {
    render(<BlockList content={content} images={[]} videos={[clip]} />)
    expect(screen.getByText('gone.mp4')).toBeInTheDocument()
  })

  it('lets a consumer wrap the rendered clip', () => {
    render(
      <BlockList
        content={content}
        images={[]}
        videos={[clip]}
        renderVideo={(_block, rendered) => (
          <div>
            {rendered}
            <span>기기의 원본 영상을 첨부해 주세요</span>
          </div>
        )}
      />,
    )
    expect(screen.getAllByText('기기의 원본 영상을 첨부해 주세요')).toHaveLength(2)
  })
})

describe('named origin rendering seams', () => {
  const image = {
    id: 'photo',
    filename: 'photo.jpg',
    width: 1024,
    height: 768,
    bytes: 1000,
    viewUrl: 'https://storage.test/photo.jpg',
  }
  const content = create(PostContentSchema, {
    title: '제목😀',
    summary: '요약😀',
    tags: ['태그😀', '태그😀'],
    blocks: [
      { type: BlockType.TEXT, content: '본문😀' },
      { type: BlockType.HEADING, level: 3, content: '소제목😀' },
      { type: BlockType.QUOTE, content: '인용😀' },
      { type: BlockType.LIST, items: ['항목😀', '항목😀'] },
      { type: BlockType.IMAGE, file: image.filename, alt: '사진 alt😀', caption: '사진 설명😀' },
      {
        type: BlockType.GALLERY,
        files: [image.filename, 'gone.jpg'],
        layout: GalleryLayout.COLLAGE,
        alt: '묶음 alt😀',
        caption: '묶음 설명😀',
      },
      { type: BlockType.VIDEO, file: 'gone.mp4', alt: '영상 alt😀', caption: '영상 설명😀' },
      {
        type: BlockType.IMAGE,
        file: 'gone.jpg',
        alt: '없는 사진 alt😀',
        caption: '없는 사진 설명😀',
      },
    ],
  })

  it('presents every canonical text field, indexed repeats and adjacent media alt without rewriting native alt', () => {
    const renderTextField = vi.fn((_field: OriginFieldLocator, text: string) => <span>{text}</span>)
    const renderAltField = vi.fn((_field: OriginFieldLocator, text: string) => (
      <p>{`대체 텍스트: ${text}`}</p>
    ))
    render(
      <BlockList
        content={content}
        images={[image]}
        renderTextField={renderTextField}
        renderAltField={renderAltField}
      />,
    )
    expect(renderTextField.mock.calls.map(([field]) => field)).toEqual(
      expect.arrayContaining([
        { kind: 'title' },
        { kind: 'summary' },
        { kind: 'tag', tagIndex: 0 },
        { kind: 'tag', tagIndex: 1 },
        { kind: 'block_content', blockIndex: 0 },
        { kind: 'block_content', blockIndex: 1 },
        { kind: 'block_content', blockIndex: 2 },
        { kind: 'block_item', blockIndex: 3, itemIndex: 0 },
        { kind: 'block_item', blockIndex: 3, itemIndex: 1 },
        { kind: 'block_caption', blockIndex: 4 },
        { kind: 'block_caption', blockIndex: 5 },
        { kind: 'block_caption', blockIndex: 6 },
        { kind: 'block_caption', blockIndex: 7 },
      ]),
    )
    expect(renderTextField).toHaveBeenCalledTimes(13)
    expect(renderAltField.mock.calls.map(([field]) => field)).toEqual(
      [4, 5, 6, 7].map((blockIndex) => ({ kind: 'block_alt', blockIndex })),
    )
    expect(screen.getByRole('img', { name: '사진 alt😀' })).toHaveAttribute('alt', '사진 alt😀')
    expect(screen.getByRole('img', { name: '묶음 alt😀 (1/2)' })).toHaveAttribute(
      'alt',
      '묶음 alt😀 (1/2)',
    )
    expect(screen.getByText('대체 텍스트: 영상 alt😀')).toBeInTheDocument()
    expect(screen.getByText('대체 텍스트: 없는 사진 alt😀')).toBeInTheDocument()
    expect(screen.getByText('없는 사진 설명😀')).toBeInTheDocument()
    expect(screen.getAllByText('묶음 설명😀')).toHaveLength(1)
  })

  it('keeps missing image evidence next to a supplied placeholder and default missing images invisible', () => {
    const { rerender } = render(<BlockList content={content} images={[image]} />)
    expect(screen.queryByText('없는 사진 설명😀')).not.toBeInTheDocument()
    rerender(
      <BlockList
        content={content}
        images={[image]}
        renderMissingImage={(block) => <p>{`없는 파일: ${block.file}`}</p>}
        renderTextField={(_field, text) => <span>{text}</span>}
        renderAltField={(_field, text) => <p>{`대체 텍스트: ${text}`}</p>}
      />,
    )
    const caption = screen.getByText('없는 사진 설명😀')
    expect(caption.parentElement?.parentElement).toContainElement(
      screen.getByText('없는 파일: gone.jpg'),
    )
    expect(caption.parentElement?.parentElement).toContainElement(
      screen.getByText('대체 텍스트: 없는 사진 alt😀'),
    )
  })

  it('preserves the exact readable strings when text inspection is toggled on and off', () => {
    const { container, rerender } = render(
      <BlockList
        content={content}
        images={[image, { ...image, id: 'gone', filename: 'gone.jpg' }]}
      />,
    )
    const plainText = container.textContent
    rerender(
      <BlockList
        content={content}
        images={[image, { ...image, id: 'gone', filename: 'gone.jpg' }]}
        renderTextField={(_field, text) => <span>{text}</span>}
      />,
    )
    expect(container.textContent).toBe(plainText)
    rerender(
      <BlockList
        content={content}
        images={[image, { ...image, id: 'gone', filename: 'gone.jpg' }]}
      />,
    )
    expect(container.textContent).toBe(plainText)
    expect(content.title).toBe('제목😀')
    expect(content.blocks[4]?.alt).toBe('사진 alt😀')
  })

  it('keeps paragraph, header tags and list item DOM nodes mounted while canonical text changes', () => {
    const { rerender } = render(<BlockList content={content} images={[]} />)
    const paragraph = screen.getByText('본문😀')
    const tag = screen.getAllByText('#태그😀')[0]
    const item = screen.getAllByText('항목😀')[0]
    const changed = create(PostContentSchema, {
      ...content,
      tags: ['새 태그😀', content.tags[1]!],
      blocks: content.blocks.map((block, index) =>
        index === 0
          ? { ...block, content: '새 본문😀' }
          : index === 3
            ? { ...block, items: ['새 항목😀', block.items[1]!] }
            : block,
      ),
    })
    rerender(<BlockList content={changed} images={[]} />)
    expect(screen.getByText('새 본문😀')).toBe(paragraph)
    expect(screen.getByText('#새 태그😀')).toBe(tag)
    expect(screen.getByText('새 항목😀')).toBe(item)
  })

  it('retains a consumer editor input and caret through a changed field and source toggle', () => {
    const renderBlock = (
      block: (typeof content.blocks)[number],
      index: number,
      rendered: ReactNode,
    ) =>
      index === 0 ? (
        <div>
          <input aria-label="본문 편집" defaultValue={block.content} />
          {rendered}
        </div>
      ) : (
        rendered
      )
    const { rerender } = render(
      <BlockList content={content} images={[]} renderBlock={renderBlock} />,
    )
    const input = screen.getByRole('textbox', { name: '본문 편집' }) as HTMLInputElement
    input.focus()
    input.setSelectionRange(2, 2)
    const changed = create(PostContentSchema, {
      ...content,
      blocks: content.blocks.map((block, index) =>
        index === 0 ? { ...block, content: '본문 바뀜😀' } : block,
      ),
    })
    rerender(
      <BlockList
        content={changed}
        images={[]}
        renderBlock={renderBlock}
        renderTextField={(_field, text) => <span>{text}</span>}
      />,
    )
    expect(screen.getByRole('textbox', { name: '본문 편집' })).toBe(input)
    expect(input).toHaveFocus()
    expect(input.selectionStart).toBe(2)
  })
})
