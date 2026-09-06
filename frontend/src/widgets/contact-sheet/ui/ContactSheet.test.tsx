import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ObservationSchema } from '@/shared/api'
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

const clip = {
  id: 'video-1',
  filename: 'clip.mp4',
  width: 1920,
  height: 1080,
  bytes: 12_000_000,
  durationMs: 65_400,
  contentType: 'video/mp4',
  viewUrl: 'https://storage.test/clip.mp4?signature=read',
}

// VIDEO-9: a clip is carded like a photo, plus the two things only a clip has — what happens,
// in order, and what was heard.
it('cards a clip with its duration, its events and its speech', () => {
  render(
    <ContactSheet
      images={[]}
      videos={[clip]}
      observations={[
        create(ObservationSchema, {
          file: 'clip.mp4',
          scene: '해변',
          events: ['파도가 친다', '아이가 뛴다'],
          speech: '좋다',
        }),
      ]}
    />,
  )

  expect(screen.getByText('clip.mp4')).toBeInTheDocument()
  expect(screen.getByText('1:05')).toBeInTheDocument()
  // The order is the information, so the events are a list and not one joined line.
  const events = screen.getByRole('list')
  expect(events).toHaveTextContent('파도가 친다')
  expect(events).toHaveTextContent('아이가 뛴다')
  expect(screen.getByText('좋다')).toBeInTheDocument()
})

// A card that grew to fifteen lines would take the strip's other cards off the screen.
it('holds a long timeline behind a disclosure', async () => {
  const user = userEvent.setup()
  render(
    <ContactSheet
      images={[]}
      videos={[clip]}
      observations={[
        create(ObservationSchema, {
          file: 'clip.mp4',
          scene: '해변',
          events: ['1', '2', '3', '4', '5', '6', '7', '8'],
        }),
      ]}
    />,
  )

  expect(screen.queryByText('8')).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /2/ }))
  expect(screen.getByText('8')).toBeInTheDocument()
})

it('says a clip is waiting until an entry exists', () => {
  render(<ContactSheet images={[]} videos={[clip]} observations={[]} />)
  expect(screen.getByText('관찰 결과 없음')).toBeInTheDocument()
})
