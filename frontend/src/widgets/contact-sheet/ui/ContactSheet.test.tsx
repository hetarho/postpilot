import { act, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { OBSERVATION_FIXTURE } from '@/test/fixtures/postContent'
import { ContactSheet } from './ContactSheet'

afterEach(() => vi.unstubAllGlobals())

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
        failure: undefined,
        postSlug: 'post-a',
        observeModel: undefined,
        writeModel: undefined,
        createdAt: '',
        updatedAt: '',
        targetLanguage: 'ko',
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

// A6: on a phone the strip is one horizontal snap carousel with a position indicator, and it never
// grows a nested VERTICAL scroller — the page still scrolls from anywhere on it.
it('is a horizontal snap carousel that reports where the reader is', () => {
  // jsdom has no IntersectionObserver; this one records its callback so the test can report which
  // card the snap has settled on, which is what the component actually listens to.
  let report: ((entries: Array<Partial<IntersectionObserverEntry>>) => void) | undefined
  const observed: Element[] = []
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(callback: (entries: Array<Partial<IntersectionObserverEntry>>) => void) {
        report = callback
      }
      observe(target: Element) {
        observed.push(target)
      }
      disconnect() {}
    },
  )

  render(<ContactSheet images={images} observations={[]} />)

  const strip = screen.getAllByRole('article')[0]!.parentElement!
  expect(strip).toHaveClass('snap-x', 'snap-mandatory', 'overflow-x-auto', 'overscroll-x-contain')
  expect(strip.className).not.toMatch(/overflow-y/)
  // Narrower than the strip on purpose, so a sliver of the next card says it scrolls; the wide
  // shape is unchanged.
  for (const card of screen.getAllByRole('article')) {
    expect(card).toHaveClass('w-carousel-card', 'shrink-0', 'snap-start', 'sm:w-60')
  }
  expect(observed).toHaveLength(2)

  expect(screen.getByRole('status')).toHaveTextContent('1 / 2')
  act(() =>
    report?.([
      { target: strip.children[1]!, isIntersecting: true, intersectionRatio: 0.9 },
      { target: strip.children[0]!, isIntersecting: true, intersectionRatio: 0.1 },
    ]),
  )
  expect(screen.getByRole('status')).toHaveTextContent('2 / 2')
})

it('shows no position indicator for a single photo', () => {
  render(<ContactSheet images={[images[0]!]} observations={[]} />)
  expect(screen.queryByRole('status')).not.toBeInTheDocument()
})
