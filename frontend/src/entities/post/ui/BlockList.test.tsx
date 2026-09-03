import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
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

  expect(screen.getByRole('heading', { name: '비 온 뒤의 제주', level: 3 })).toHaveClass('text-lg')
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
