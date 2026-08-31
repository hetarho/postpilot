import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UploadItem } from '../model/upload-batch'
import { PhotoStrip } from './PhotoStrip'

const FAILED_UPLOAD: UploadItem = {
  id: 'failed-1',
  name: 'IMG_1.jpg',
  filename: 'IMG_1.jpg',
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
