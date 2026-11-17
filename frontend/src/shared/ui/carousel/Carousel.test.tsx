import { act, cleanup, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, expect, it, vi } from 'vitest'
import { Carousel } from './Carousel'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

/** jsdom has no IntersectionObserver; this one records its callback so the test can report which
 *  slide the snap settled on. */
function stubObserver() {
  const observer = {
    report: undefined as ((entries: Array<Partial<IntersectionObserverEntry>>) => void) | undefined,
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

function Harness({ onIndexChange }: { onIndexChange?: (index: number) => void }) {
  const [index, setIndex] = useState(0)
  return (
    <>
      <p role="status">{index}</p>
      <button type="button" onClick={() => setIndex(2)}>
        last
      </button>
      <Carousel
        index={index}
        label="slides"
        onIndexChange={(next) => {
          setIndex(next)
          onIndexChange?.(next)
        }}
      >
        <p key="a">A</p>
        <p key="b">B</p>
        <p key="c">C</p>
      </Carousel>
    </>
  )
}

it('is one snap strip of full-width slides that reports the slide a swipe settled on', () => {
  const observer = stubObserver()
  const scrollTo = vi.fn()
  Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: scrollTo })
  const onIndexChange = vi.fn()
  render(<Harness onIndexChange={onIndexChange} />)
  const strip = screen.getByRole('group', { name: 'slides' })
  expect(strip).toHaveClass('snap-x', 'snap-mandatory', 'overflow-x-auto', 'overscroll-x-contain')
  expect(observer.observed).toEqual(Array.from(strip.children))
  expect(strip.children[0]).toHaveClass('w-full', 'shrink-0', 'snap-center')

  // A slide only partly on screen is still travelling; the one mostly on screen has settled.
  act(() =>
    observer.report?.([
      { target: observer.observed[1]!, isIntersecting: true, intersectionRatio: 0.4 },
    ]),
  )
  expect(onIndexChange).not.toHaveBeenCalled()
  act(() =>
    observer.report?.([
      { target: observer.observed[1]!, isIntersecting: true, intersectionRatio: 0.9 },
    ]),
  )
  expect(onIndexChange).toHaveBeenCalledExactlyOnceWith(1)
  expect(screen.getByRole('status')).toHaveTextContent('1')
  // The strip is already there, so the reported index does not scroll it again.
  expect(scrollTo).not.toHaveBeenCalled()
})

it('scrolls to an index the caller sets, once', () => {
  const observer = stubObserver()
  const scrollTo = vi.fn()
  Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: scrollTo })
  const onIndexChange = vi.fn()
  render(<Harness onIndexChange={onIndexChange} />)
  act(() => screen.getByRole('button', { name: 'last' }).click())
  expect(scrollTo).toHaveBeenCalledTimes(1)
  // The snap arriving at the slide it was sent to is not a new choice.
  act(() =>
    observer.report?.([
      { target: observer.observed[2]!, isIntersecting: true, intersectionRatio: 1 },
    ]),
  )
  expect(onIndexChange).not.toHaveBeenCalled()
  expect(scrollTo).toHaveBeenCalledTimes(1)
})
