import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UploadItem } from '../model/upload-batch'
import { PhotoStrip } from './PhotoStrip'

const FAILED_UPLOAD: UploadItem = {
  id: 'failed-1',
  name: 'IMG_1.jpg',
  filename: 'IMG_1.jpg',
  attachment: 'photo' as const,
  status: 'failed',
  failure: 'network',
}

describe('PhotoStrip', () => {
  it('keeps retry and dismiss as separate named actions', async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()
    const onDismiss = vi.fn()
    render(
      <PhotoStrip
        images={[]}
        items={[FAILED_UPLOAD]}
        onDelete={vi.fn()}
        onRetry={onRetry}
        onDismiss={onDismiss}
      />,
    )

    await user.click(screen.getByRole('button', { name: '다시 시도' }))
    await user.click(screen.getByRole('button', { name: '지우기' }))

    expect(onRetry).toHaveBeenCalledWith('failed-1')
    expect(onDismiss).toHaveBeenCalledWith('failed-1')
  })

  it('renders the structured RPC refusal instead of a code-derived category', () => {
    render(
      <PhotoStrip
        images={[]}
        items={[
          {
            ...FAILED_UPLOAD,
            failure: 'duplicate-filename',
            appFailure: {
              reason: 'POST_FILENAME_TAKEN',
              params: { filename: 'IMG_1.jpg' },
            },
          },
        ]}
        onDelete={vi.fn()}
        onRetry={vi.fn()}
        onDismiss={vi.fn()}
      />,
    )

    expect(screen.getByText('같은 이름의 사진이 이미 있어요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
    expect(document.body).not.toHaveTextContent('private backend prose')
  })
})

describe('the strip with videos', () => {
  const clip = {
    id: 'video-1',
    filename: 'clip.mp4',
    width: 1920,
    height: 1080,
    bytes: 12_000_000,
    durationMs: 65_400,
    contentType: 'video/mp4',
    viewUrl: 'https://storage.test/clip.mp4',
  }

  const strip = (props: Partial<Parameters<typeof PhotoStrip>[0]> = {}) =>
    render(
      <PhotoStrip
        images={[]}
        videos={[clip]}
        items={[]}
        onDelete={() => {}}
        onRetry={() => {}}
        onDismiss={() => {}}
        {...props}
      />,
    )

  // The duration badge is what tells a clip from a photo at a glance, so it is text and not
  // only the play glyph (VIDEO-7).
  it('shows a clip after the photos with its duration', () => {
    strip()
    expect(screen.getByText('1:05')).toBeInTheDocument()
  })

  // The same delete affordance a photo has, through the same confirm sheet.
  it('deletes a clip through the same confirmation', async () => {
    const user = userEvent.setup()
    const onDeleteVideo = vi.fn()
    strip({ onDeleteVideo })

    await user.click(screen.getByRole('button', { name: 'clip.mp4 삭제' }))
    await user.click(screen.getByRole('button', { name: '삭제' }))
    expect(onDeleteVideo).toHaveBeenCalledWith(clip)
  })

  // A 200 MB PUT over mobile takes a minute; a card that only said 올리는 중 would read as stuck.
  it('reports a percentage while a clip is uploading', () => {
    strip({
      videos: [],
      items: [
        {
          id: 'upload-1',
          name: 'clip.mp4',
          filename: 'clip.mp4',
          attachment: 'video' as const,
          status: 'uploading' as const,
          progress: 42,
        },
      ],
    })
    expect(screen.getByText('올리는 중 42%')).toBeInTheDocument()
  })
})
