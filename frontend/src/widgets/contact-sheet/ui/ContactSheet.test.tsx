import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { OBSERVATION_FIXTURE } from '@/test/fixtures/postContent'
import { ContactSheet } from './ContactSheet'

const images = [
  {
    id: 'image-1',
    filename: 'IMG_1.jpg',
    width: 1024,
    height: 768,
    bytes: 200_000,
    viewUrl: 'https://storage.test/IMG_1.jpg?signature=read',
  },
  {
    id: 'image-2',
    filename: 'IMG_2.jpg',
    width: 1024,
    height: 768,
    bytes: 200_000,
    viewUrl: 'https://storage.test/IMG_2.jpg?signature=read',
  },
]

it('shows persisted observations and waiting photos during observe', () => {
  render(
    <ContactSheet
      images={images}
      observations={[OBSERVATION_FIXTURE]}
      activeJob={{
        id: 'job-1',
        kind: 'generate',
        status: 'running',
        stage: 'observe',
        progressDone: 1,
        progressTotal: 2,
        error: '',
        postSlug: 'post-a',
        observeModel: undefined,
        writeModel: undefined,
        createdAt: '',
        updatedAt: '',
      }}
    />,
  )

  expect(screen.getByText('비가 그친 바닷가')).toBeInTheDocument()
  expect(screen.getByText('차분함')).toBeInTheDocument()
  expect(screen.getByText('JEJU')).toBeInTheDocument()
  expect(screen.getByText('우산, 파도')).toBeInTheDocument()
  expect(screen.getByText('관찰 대기')).toBeInTheDocument()
  expect(screen.getByRole('img', { name: 'IMG_1.jpg 관찰 사진' })).toHaveAttribute(
    'src',
    images[0]?.viewUrl,
  )
})

it('never uses a local upload preview as the contact-sheet source', () => {
  render(
    <ContactSheet
      images={[{ ...images[0]!, viewUrl: 'blob:local-upload-preview' }]}
      observations={[]}
    />,
  )

  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.getByText('사진 주소를 준비하는 중…')).toBeInTheDocument()
})
