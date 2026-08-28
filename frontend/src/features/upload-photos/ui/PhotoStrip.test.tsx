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
})
