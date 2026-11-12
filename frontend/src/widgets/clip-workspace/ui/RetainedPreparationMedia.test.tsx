import { render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { RetainedPreparationMedia } from './RetainedPreparationMedia'

it.each(['story', 'style', 'review', 'refine', 'focused work'])(
  'pauses retained footage on leaving sources for %s without replacing it or restarting it',
  () => {
    const view = render(
      <RetainedPreparationMedia active>
        <video aria-label="Original" />
      </RetainedPreparationMedia>,
    )
    const media = screen.getByLabelText('Original') as HTMLVideoElement
    Object.defineProperty(media, 'paused', { configurable: true, value: false })
    media.currentTime = 4
    const play = vi.spyOn(media, 'play').mockResolvedValue()
    const pause = vi.spyOn(media, 'pause').mockImplementation(() => {})
    view.rerender(
      <RetainedPreparationMedia active={false}>
        <video aria-label="Original" />
      </RetainedPreparationMedia>,
    )
    expect(pause).toHaveBeenCalledOnce()
    view.rerender(
      <RetainedPreparationMedia active>
        <video aria-label="Original" />
      </RetainedPreparationMedia>,
    )
    expect(screen.getByLabelText('Original')).toBe(media)
    expect(media.currentTime).toBe(4)
    expect(play).not.toHaveBeenCalled()
    expect(pause).toHaveBeenCalledOnce()
  },
)
