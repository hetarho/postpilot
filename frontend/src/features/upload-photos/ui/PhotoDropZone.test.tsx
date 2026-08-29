import { fireEvent, render, screen } from '@testing-library/react'
import { PhotoDropZone } from './PhotoDropZone'

const file = new File(['x'], 'a.jpg', { type: 'image/jpeg' })
const fileTransfer = (files: File[] = [file]) => ({
  dataTransfer: { types: ['Files'], files, dropEffect: 'none' },
})

describe('PhotoDropZone', () => {
  it('hands dropped files to onFiles and clears the cue', () => {
    const onFiles = vi.fn()
    render(
      <PhotoDropZone onFiles={onFiles}>
        <button>child</button>
      </PhotoDropZone>,
    )
    const zone = screen.getByText('child').parentElement!

    fireEvent.dragEnter(zone, fileTransfer())
    expect(screen.getByText('여기에 놓으면 사진이 추가돼요')).toBeInTheDocument()

    fireEvent.drop(zone, fileTransfer())
    expect(onFiles).toHaveBeenCalledWith([file])
    expect(screen.queryByText('여기에 놓으면 사진이 추가돼요')).not.toBeInTheDocument()
  })

  it('keeps the cue while the pointer crosses children, and drops it on leaving the zone', () => {
    render(
      <PhotoDropZone onFiles={vi.fn()}>
        <button>child</button>
      </PhotoDropZone>,
    )
    const child = screen.getByText('child')
    const zone = child.parentElement!

    fireEvent.dragEnter(zone, fileTransfer())
    fireEvent.dragEnter(child, fileTransfer())
    fireEvent.dragLeave(child, fileTransfer())
    expect(screen.getByText('여기에 놓으면 사진이 추가돼요')).toBeInTheDocument()

    fireEvent.dragLeave(zone, fileTransfer())
    expect(screen.queryByText('여기에 놓으면 사진이 추가돼요')).not.toBeInTheDocument()
  })

  it('ignores drags that carry no files', () => {
    const onFiles = vi.fn()
    render(
      <PhotoDropZone onFiles={onFiles}>
        <button>child</button>
      </PhotoDropZone>,
    )
    const zone = screen.getByText('child').parentElement!
    const text = { dataTransfer: { types: ['text/plain'], files: [], dropEffect: 'none' } }

    fireEvent.dragEnter(zone, text)
    expect(screen.queryByText('여기에 놓으면 사진이 추가돼요')).not.toBeInTheDocument()
    fireEvent.drop(zone, text)
    expect(onFiles).not.toHaveBeenCalled()
  })

  it('does nothing while disabled', () => {
    const onFiles = vi.fn()
    render(
      <PhotoDropZone onFiles={onFiles} disabled>
        <button>child</button>
      </PhotoDropZone>,
    )
    const zone = screen.getByText('child').parentElement!

    fireEvent.dragEnter(zone, fileTransfer())
    expect(screen.queryByText('여기에 놓으면 사진이 추가돼요')).not.toBeInTheDocument()
    fireEvent.drop(zone, fileTransfer())
    expect(onFiles).not.toHaveBeenCalled()
  })
})
