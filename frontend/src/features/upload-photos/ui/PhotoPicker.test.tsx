import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PhotoPicker } from './PhotoPicker'

describe('PhotoPicker', () => {
  it('keeps the native file input keyboard reachable behind the visible label', async () => {
    const user = userEvent.setup()
    render(<PhotoPicker onFiles={vi.fn()} />)

    await user.tab()

    expect(screen.getByLabelText('사진 추가')).toHaveFocus()
  })

  it('removes the disabled trigger from keyboard navigation', () => {
    render(<PhotoPicker onFiles={vi.fn()} disabled />)

    expect(screen.getByLabelText('사진 추가')).toBeDisabled()
  })
})
