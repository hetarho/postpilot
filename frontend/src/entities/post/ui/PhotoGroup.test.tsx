import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { BlockSchema, BlockType, GalleryLayout, PostContentSchema } from '@/shared/api'
import type { PostImage } from '@/entities/image/@x/post'
import { BlockList } from './BlockList'
import { PhotoGroup } from './PhotoGroup'

const photo = (filename: string): PostImage => ({
  id: filename,
  filename,
  width: 1024,
  height: 768,
  bytes: 1000,
  viewUrl: `https://storage.test/${filename}`,
})
const photos = (...filenames: string[]) => new Map(filenames.map((name) => [name, photo(name)]))
const group = (layout: GalleryLayout, files: string[], caption = '', alt = '') =>
  create(BlockSchema, { type: BlockType.GALLERY, files, layout, caption, alt })

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('a 콜라주 group', () => {
  it('lays its photos side by side in one grid with the caption once under it', () => {
    render(
      <PhotoGroup
        block={group(GalleryLayout.COLLAGE, ['a.jpg', 'b.jpg', 'c.jpg'], '창가 자리', '카페 안')}
        images={photos('a.jpg', 'b.jpg', 'c.jpg')}
      />,
    )
    const images = screen.getAllByRole('img')
    expect(images).toHaveLength(3)
    expect(images[0]!.closest('.grid')).toHaveClass('grid-cols-3')
    // The group's one alt, told apart by position.
    expect(images.map((image) => image.getAttribute('alt'))).toEqual([
      '카페 안 (1/3)',
      '카페 안 (2/3)',
      '카페 안 (3/3)',
    ])
    expect(screen.getAllByText('창가 자리')).toHaveLength(1)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('lays two photos in two columns and three in three, in one row', () => {
    const { rerender } = render(
      <PhotoGroup
        block={group(GalleryLayout.COLLAGE, ['a.jpg', 'b.jpg'])}
        images={photos('a.jpg', 'b.jpg')}
      />,
    )
    expect(screen.getAllByRole('img')[0]!.closest('.grid')).toHaveClass('grid-cols-2')
    rerender(
      <PhotoGroup
        block={group(GalleryLayout.COLLAGE, ['a.jpg', 'b.jpg', 'c.jpg'])}
        images={photos('a.jpg', 'b.jpg', 'c.jpg')}
      />,
    )
    expect(screen.getAllByRole('img')[0]!.closest('.grid')).toHaveClass('grid-cols-3')
  })

  it('reads an unspecified layout as a collage and falls back to the filename for alt', () => {
    render(
      <PhotoGroup
        block={group(GalleryLayout.UNSPECIFIED, ['a.jpg', 'b.jpg'])}
        images={photos('a.jpg', 'b.jpg')}
      />,
    )
    expect(screen.getByRole('img', { name: 'a.jpg (1/2)' }).closest('.grid')).not.toBeNull()
  })

  it('holds the cell of a photo the post no longer has, naming the file', () => {
    render(
      <PhotoGroup
        block={group(GalleryLayout.COLLAGE, ['a.jpg', 'gone.jpg'])}
        images={photos('a.jpg')}
      />,
    )
    expect(screen.getAllByRole('img')).toHaveLength(1)
    expect(screen.getByText('gone.jpg').closest('.aspect-square')).not.toBeNull()
  })

  it('lets a consumer wrap each photo and replace the caption', () => {
    render(
      <PhotoGroup
        block={group(GalleryLayout.COLLAGE, ['a.jpg', 'b.jpg'], '설명')}
        images={photos('a.jpg', 'b.jpg')}
        renderPhoto={(file, position, _image, fit) => (
          <button type="button" aria-label={`${position + 1}번 ${file} 복사 (${fit})`} />
        )}
        renderCaption={(block) => <p>{`캡션 복사: ${block.caption}`}</p>}
      />,
    )
    expect(screen.getByRole('button', { name: '2번 b.jpg 복사 (cover)' })).toBeInTheDocument()
    expect(screen.getByText('캡션 복사: 설명')).toBeInTheDocument()
    expect(screen.queryByText('설명')).not.toBeInTheDocument()
  })
})

describe('a 슬라이드 group', () => {
  // jsdom has no IntersectionObserver; this one records its callback so the test can report which
  // slide the snap settled on, which is what the strip listens to.
  function stubObserver() {
    const observer = {
      report: undefined as
        ((entries: Array<Partial<IntersectionObserverEntry>>) => void) | undefined,
      observed: [] as Element[],
    }
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(callback: (entries: Array<Partial<IntersectionObserverEntry>>) => void) {
          observer.report = callback
        }
        observe(target: Element) {
          observer.observed.push(target)
        }
        disconnect() {}
      },
    )
    return observer
  }

  it('is one snap strip with a position and previous/next buttons disabled at the ends', () => {
    const observer = stubObserver()
    render(
      <PhotoGroup
        block={group(GalleryLayout.SLIDE, ['a.jpg', 'b.jpg', 'c.jpg'], '넘겨 보기')}
        images={photos('a.jpg', 'b.jpg', 'c.jpg')}
      />,
    )
    const strip = screen.getAllByRole('img')[0]!.parentElement!.parentElement!
    expect(strip).toHaveClass('snap-x', 'snap-mandatory', 'overflow-x-auto', 'overscroll-x-contain')
    expect(observer.observed).toHaveLength(3)
    expect(screen.getByRole('status')).toHaveTextContent('1 / 3')
    expect(screen.getByRole('button', { name: '이전 사진' })).toBeDisabled()
    expect(screen.getByText('넘겨 보기')).toBeInTheDocument()

    act(() =>
      observer.report?.([
        { target: observer.observed[2]!, isIntersecting: true, intersectionRatio: 0.9 },
      ]),
    )
    expect(screen.getByRole('status')).toHaveTextContent('3 / 3')
    expect(screen.getByRole('button', { name: '다음 사진' })).toBeDisabled()
  })

  it('moves the strip one slide with the buttons', () => {
    stubObserver()
    const scrollTo = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
      configurable: true,
      value: scrollTo,
    })
    render(
      <PhotoGroup
        block={group(GalleryLayout.SLIDE, ['a.jpg', 'b.jpg'])}
        images={photos('a.jpg', 'b.jpg')}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: '다음 사진' }))
    expect(scrollTo).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('status')).toHaveTextContent('2 / 2')
    fireEvent.click(screen.getByRole('button', { name: '이전 사진' }))
    expect(screen.getByRole('status')).toHaveTextContent('1 / 2')
  })
})

it('renders a GALLERY block of the block array as one group', () => {
  const content = create(PostContentSchema, {
    title: '카페',
    blocks: [
      create(BlockSchema, { type: BlockType.TEXT, content: '도착했다' }),
      group(GalleryLayout.COLLAGE, ['a.jpg', 'b.jpg'], '창가'),
    ],
  })
  render(<BlockList content={content} images={[photo('a.jpg'), photo('b.jpg')]} />)
  const article = screen.getByRole('article')
  expect(within(article).getAllByRole('img')).toHaveLength(2)
  expect(within(article).getByText('창가')).toBeInTheDocument()
})

it('uses the group caption seam once and keeps the group alt native when evidence is inspected', () => {
  const content = create(PostContentSchema, {
    title: '사진 근거',
    blocks: [
      group(GalleryLayout.COLLAGE, ['a.jpg', 'gone.jpg'], '보이는 캡션😀', '그룹 대체 텍스트😀'),
    ],
  })
  const renderTextField = vi.fn((_field, text: string) => <span>{text}</span>)
  const renderAltField = vi.fn((_field, text: string) => <p>{`alt 근거: ${text}`}</p>)
  render(
    <BlockList
      content={content}
      images={[photo('a.jpg')]}
      renderTextField={renderTextField}
      renderAltField={renderAltField}
    />,
  )
  expect(renderTextField).toHaveBeenCalledWith(
    { kind: 'block_caption', blockIndex: 0 },
    '보이는 캡션😀',
  )
  expect(screen.getAllByText('보이는 캡션😀')).toHaveLength(1)
  expect(screen.getByRole('img', { name: '그룹 대체 텍스트😀 (1/2)' })).toHaveAttribute(
    'alt',
    '그룹 대체 텍스트😀 (1/2)',
  )
  expect(renderAltField).toHaveBeenCalledWith(
    { kind: 'block_alt', blockIndex: 0 },
    '그룹 대체 텍스트😀',
  )
  expect(screen.getByText('alt 근거: 그룹 대체 텍스트😀')).toBeInTheDocument()
  expect(screen.getByText('gone.jpg')).toBeInTheDocument()
})

// POST-107: the reading view shows a turned photo upright — a single photo in a frame of the
// turned shape, a group's cell turned in place.
it('shows turned photos upright in the reading view', () => {
  const content = create(PostContentSchema, {
    title: '회전',
    blocks: [
      create(BlockSchema, { type: BlockType.IMAGE, file: 'a.jpg', alt: '한 장' }),
      group(GalleryLayout.COLLAGE, ['b.jpg', 'c.jpg'], '', '묶음'),
    ],
  })
  render(
    <BlockList
      content={content}
      images={[
        { ...photo('a.jpg'), width: 400, height: 300, rotation: 90 },
        { ...photo('b.jpg'), rotation: 180 },
        photo('c.jpg'),
      ]}
    />,
  )
  const single = screen.getByRole('img', { name: '한 장' })
  expect(single.parentElement).toHaveStyle({ aspectRatio: '300 / 400' })
  expect(screen.getByRole('img', { name: '묶음 (1/2)' })).toHaveStyle({
    transform: 'rotate(180deg)',
  })
  expect(screen.getByRole('img', { name: '묶음 (2/2)' }).getAttribute('style')).toBeNull()
})
