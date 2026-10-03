import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { RotatedImage } from './RotatedImage'

// POST-107: one primitive turns a photo for every surface. A filled box turns the picture in
// place; a photo that keeps its own shape swaps the box's sides for a quarter turn.
describe('RotatedImage', () => {
  it('draws an unturned photo as a plain image, and an unknown turn as none', () => {
    const { rerender } = render(
      <RotatedImage fit="natural" src="a.jpg" alt="a" width={4} height={3} />,
    )
    expect(screen.getByRole('img', { name: 'a' }).getAttribute('style')).toBeNull()
    rerender(<RotatedImage fit="natural" rotation={45} src="a.jpg" alt="a" width={4} height={3} />)
    expect(screen.getByRole('img', { name: 'a' }).getAttribute('style')).toBeNull()
  })

  it('turns a filled image in place for every quarter turn', () => {
    for (const turn of [90, 180, 270]) {
      const { unmount } = render(<RotatedImage fit="fill" rotation={turn} src="a.jpg" alt="a" />)
      expect(screen.getByRole('img', { name: 'a' })).toHaveStyle({
        transform: `rotate(${turn}deg)`,
      })
      unmount()
    }
  })

  it('turns a natural photo half a turn without changing its box', () => {
    render(<RotatedImage fit="natural" rotation={180} src="a.jpg" alt="a" width={4} height={3} />)
    const image = screen.getByRole('img', { name: 'a' })
    expect(image).toHaveStyle({ transform: 'rotate(180deg)' })
    expect(image.parentElement?.tagName).not.toBe('SPAN')
  })

  it('gives a quarter-turned natural photo a frame of the turned shape', () => {
    render(
      <RotatedImage
        fit="natural"
        rotation={90}
        src="a.jpg"
        alt="a"
        width={400}
        height={300}
        className="max-h-media-view w-full"
        frameClassName="rounded-lg"
      />,
    )
    const image = screen.getByRole('img', { name: 'a' })
    const frame = image.parentElement!
    // Stored 400×300, shown 300×400: the frame's ratio is 300/400, the image as wide as the frame
    // is tall (400/300 of its width) and turned about the centre.
    expect(frame).toHaveStyle({ aspectRatio: '300 / 400' })
    expect(frame).toHaveClass('rounded-lg', 'overflow-hidden')
    expect(image).toHaveStyle({ transform: 'translate(-50%, -50%) rotate(90deg)' })
    expect(image.style.width).toBe(`${(400 / 300) * 100}%`)
    expect(image).toHaveClass('max-h-none', 'max-w-none')
  })

  it('narrows a capped frame to keep the turned shape inside the cap', () => {
    render(
      <RotatedImage
        fit="natural"
        rotation={270}
        src="a.jpg"
        alt="a"
        width={400}
        height={300}
        maxFrameHeight="var(--spacing-media-view)"
      />,
    )
    expect(screen.getByRole('img', { name: 'a' }).parentElement!.style.width).toBe(
      'min(100%, calc(var(--spacing-media-view) * 0.75))',
    )
  })
})
